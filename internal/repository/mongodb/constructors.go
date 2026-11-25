package mongodb

import (
	"go.mongodb.org/mongo-driver/v2/mongo"

	"subman/internal/payment"
	"subman/internal/repository"
	"subman/internal/subscription"
	"subman/internal/user"
)

// The domain UUIDs are persisted as strings (see the bson.go mapping in
// each domain package), so the typed repositories are keyed by the string
// form of the ID — the exact value stored as the document _id.

var (
	_ repository.Repository[string, user.User]                  = NewUserRepository(nil)
	_ repository.Repository[string, *payment.Card]              = NewCardRepository(nil)
	_ repository.Repository[string, *subscription.Subscription] = NewSubscriptionRepository(nil)
)

// NewUserRepository wires the generic repository over the User aggregate.
func NewUserRepository(coll *mongo.Collection) *Repository[string, user.User] {
	return NewRepository(coll,
		func(u user.User) string { return u.ID().String() },
		func() *user.User { return &user.User{} },
	)
}

// NewCardRepository wires the generic repository over the Card payment
// method. Cards are pointer-based in the domain, so newEntity allocates a
// slot for a *Card.
func NewCardRepository(coll *mongo.Collection) *Repository[string, *payment.Card] {
	return NewRepository(coll,
		func(c *payment.Card) string { return c.ID().String() },
		func() **payment.Card {
			card := &payment.Card{}
			return &card
		},
	)
}

// NewSubscriptionRepository wires the generic repository over the
// Subscription aggregate. Subscriptions are pointer-based in the domain,
// so newEntity allocates a slot for a *Subscription.
func NewSubscriptionRepository(coll *mongo.Collection) *Repository[string, *subscription.Subscription] {
	return NewRepository(coll,
		func(s *subscription.Subscription) string { return s.ID().String() },
		func() **subscription.Subscription {
			sub := &subscription.Subscription{}
			return &sub
		},
	)
}
