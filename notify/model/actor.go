package model

// ActorType records what kind of principal made a request, derived from the
// token's `type` claim.
type ActorType string

const (
	ActorTypeUser           ActorType = "USER"
	ActorTypeServiceAccount ActorType = "SERVICE_ACCOUNT"
)
