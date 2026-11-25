package payment

import (
	"time"

	"github.com/google/uuid"
)

// PaymentMethod is a user's record of how a Subscription is paid.
// Optional metadata for bookkeeping only, never payment processing.
// Payment methods have identity and live in a user's Wallet.
type PaymentMethod interface {
	ID() uuid.UUID
	OwnerID() uuid.UUID
	IsActiveAt(now time.Time) bool
	Remove(at time.Time)
}
