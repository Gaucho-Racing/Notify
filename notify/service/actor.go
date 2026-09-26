package service

import "github.com/gaucho-racing/notify/notify/model"

// Actor is the resolved principal behind a request: which entity acted, which
// application's token they presented, and what kind of credential it was.
type Actor struct {
	EntityID string
	ClientID string
	Type     model.ActorType
}
