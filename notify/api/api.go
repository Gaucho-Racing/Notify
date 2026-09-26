package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gaucho-racing/notify/notify/config"
	"github.com/gaucho-racing/notify/notify/model"
	"github.com/gaucho-racing/notify/notify/pkg/logger"
	"github.com/gaucho-racing/notify/notify/pkg/sentinel"
	"github.com/gaucho-racing/notify/notify/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

const shutdownTimeout = 15 * time.Second

// Run serves until ctx is cancelled, then drains open requests so a send that
// is mid-transaction during a deploy still gets its response.
func Run(ctx context.Context) {
	router := InitializeRouter()
	InitializeRoutes(router)
	server := &http.Server{Addr: ":" + config.Port, Handler: router}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.SugarLogger.Errorf("Failed to shut down server cleanly: %v", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.SugarLogger.Fatalf("Failed to start server: %v", err)
	}
}

func InitializeRouter() *gin.Engine {
	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization", "Idempotency-Key"},
		MaxAge:           12 * time.Hour,
		AllowCredentials: false,
	}))
	r.Use(AuthChecker())
	r.Use(UnauthorizedPanicHandler())
	return r
}

func InitializeRoutes(router *gin.Engine) {
	router.GET("/ping", Ping)

	router.POST("/auth/login", LoginWithSentinel)
	router.GET("/auth/session", GetSession)
	router.POST("/auth/refresh", RefreshSession)
	router.POST("/auth/logout", Logout)
	router.GET("/users/@me", GetCurrentUser)

	router.GET("/groups", ListSentinelGroups)
	router.GET("/applications", ListSentinelApplications)
	router.POST("/applications/resolve", ResolveApplications)
	router.POST("/identities/resolve", ResolveIdentities)

	router.GET("/policies", ListAppPolicies)
	router.GET("/policies/:clientID", GetAppPolicy)
	router.PUT("/policies/:clientID", UpsertAppPolicy)
	router.DELETE("/policies/:clientID", DeleteAppPolicy)

	router.POST("/messages", SendMessage)
	router.GET("/messages", ListMessages)
	router.GET("/messages/:messageID", GetMessage)

	router.GET("/deliveries", ListDeliveries)
	router.POST("/deliveries/:deliveryID/retry", RetryDelivery)
}

func AuthChecker() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authRouteSkipsTokenValidation(c.Request.URL.Path) {
			c.Next()
			return
		}

		token := ""
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if token != "" {
			claims, err := sentinel.ValidateToken(token)
			if err != nil {
				logger.SugarLogger.Errorln("Failed to validate token: " + err.Error())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
				return
			}
			setAuthContext(c, token, claims)
		}
		c.Next()
	}
}

func authRouteSkipsTokenValidation(path string) bool {
	return path == "/auth/login" ||
		path == "/auth/refresh" ||
		path == "/auth/logout"
}

func UnauthorizedPanicHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				if err == "Unauthorized" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "you are not authorized to access this resource"})
					return
				}
				logger.SugarLogger.Errorf("Unexpected panic: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprint(err)})
			}
		}()
		c.Next()
	}
}

func Require(c *gin.Context, condition bool) {
	if !condition {
		panic("Unauthorized")
	}
}

func Any(conditions ...bool) bool {
	for _, condition := range conditions {
		if condition {
			return true
		}
	}
	return false
}

func All(conditions ...bool) bool {
	for _, condition := range conditions {
		if !condition {
			return false
		}
	}
	return true
}

func RequestTokenExists(c *gin.Context) bool {
	_, exists := c.Get("Auth-Token")
	return exists
}

func RequestTokenHasGroupName(c *gin.Context, groupName string) bool {
	for _, tokenGroup := range GetRequestTokenGroupNames(c) {
		if tokenGroup == groupName {
			return true
		}
	}
	return false
}

// RequestTokenIsFirstParty reports whether the token was minted for Notify's
// own Sentinel application, i.e. a human signed in through the web UI. Every
// Sentinel token carries exactly one audience — the client_id of the
// application it was issued for — so the audience is what separates Notify's
// own control plane from a calling application's data plane.
func RequestTokenIsFirstParty(c *gin.Context) bool {
	if config.SentinelClientID == "" {
		return false
	}
	return GetRequestTokenClientID(c) == config.SentinelClientID
}

const AdminGroupName = "NotifyAdmins"

// RequestTokenIsAdmin requires both a token minted for Notify's own OAuth
// client and membership in NotifyAdmins, so an application token can never
// reach the control plane regardless of its entity's group memberships.
func RequestTokenIsAdmin(c *gin.Context) bool {
	if !RequestTokenExists(c) {
		return false
	}
	return RequestTokenIsFirstParty(c) && RequestTokenHasGroupName(c, AdminGroupName)
}

// RequestTokenCanAccessClient gates reads of an application's messages,
// deliveries, and stats: admins see every application, and an application
// sees only its own.
func RequestTokenCanAccessClient(c *gin.Context, clientID string) bool {
	if !RequestTokenExists(c) {
		return false
	}
	if RequestTokenIsAdmin(c) {
		return true
	}
	return !RequestTokenIsFirstParty(c) && clientID != "" && GetRequestTokenClientID(c) == clientID
}

func GetRequestToken(c *gin.Context) string {
	token, _ := c.Get("Auth-Token")
	return contextString(token)
}

func GetRequestTokenScopes(c *gin.Context) string {
	scopes, _ := c.Get("Auth-Scope")
	return contextString(scopes)
}

func GetRequestTokenAudience(c *gin.Context) string {
	audience, _ := c.Get("Auth-Audience")
	return contextString(audience)
}

// GetRequestTokenClientID is the audience under the name it actually carries:
// the Sentinel client_id of the application the token was issued for.
func GetRequestTokenClientID(c *gin.Context) string {
	return GetRequestTokenAudience(c)
}

func requestActor(c *gin.Context) service.Actor {
	return service.Actor{
		EntityID: GetRequestTokenEntityID(c),
		ClientID: GetRequestTokenClientID(c),
		Type:     GetRequestActorType(c),
	}
}

// GetRequestActorType classifies the caller from Sentinel's `type` claim,
// which is set to service_account on service-account tokens and absent on
// user tokens.
func GetRequestActorType(c *gin.Context) model.ActorType {
	if claimString(GetRequestTokenClaims(c), "type") == "service_account" {
		return model.ActorTypeServiceAccount
	}
	return model.ActorTypeUser
}

func GetRequestTokenClaims(c *gin.Context) map[string]interface{} {
	claims, exists := c.Get("Auth-Claims")
	if !exists {
		return nil
	}
	value, ok := claims.(map[string]interface{})
	if !ok {
		return nil
	}
	return value
}

func GetRequestTokenEntityID(c *gin.Context) string {
	entityID, _ := c.Get("Auth-EntityID")
	return contextString(entityID)
}

func GetRequestTokenUserID(c *gin.Context) string {
	return claimString(GetRequestTokenClaims(c), "user_id")
}

func GetRequestTokenGroupNames(c *gin.Context) []string {
	return claimStringSlice(GetRequestTokenClaims(c), "groups")
}

func setAuthContext(c *gin.Context, token string, claims map[string]interface{}) {
	c.Set("Auth-Token", token)
	c.Set("Auth-Claims", claims)
	c.Set("Auth-EntityID", claimString(claims, "sub"))
	c.Set("Auth-Scope", claimString(claims, "scope"))
	c.Set("Auth-UserID", claimString(claims, "user_id"))
	audiences := claimStringSlice(claims, "aud")
	if len(audiences) > 0 {
		c.Set("Auth-Audience", audiences[0])
	}
}

func contextString(value interface{}) string {
	if value == nil {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func claimString(claims map[string]interface{}, key string) string {
	if claims == nil {
		return ""
	}
	value, ok := claims[key].(string)
	if !ok {
		return ""
	}
	return value
}

func claimStringSlice(claims map[string]interface{}, key string) []string {
	if claims == nil {
		return []string{}
	}
	switch value := claims[key].(type) {
	case []string:
		return value
	case []interface{}:
		result := make([]string, 0, len(value))
		for _, item := range value {
			if str, ok := item.(string); ok && str != "" {
				result = append(result, str)
			}
		}
		return result
	case string:
		if value == "" {
			return []string{}
		}
		return []string{value}
	default:
		return []string{}
	}
}
