package payment

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CardType is the debit or credit kind of a Card.
type CardType string

const (
	Debit  CardType = "debit"
	Credit CardType = "credit"
)

// Card is a PaymentMethod of debit or credit type with a month-granular
// expiry date, so affected Subscriptions can be surfaced when it expires.
// Expiry is derived from the expiry date, never a stored status.
type Card struct {
	id          uuid.UUID
	ownerID     uuid.UUID
	name        string
	cardType    CardType
	expiryYear  int
	expiryMonth time.Month
	removedAt   *time.Time
}

// NewCard validates and constructs a Card. The expiry is not required to be
// in the future: a user may track an already-expired card.
func NewCard(ownerID uuid.UUID, name string, cardType CardType, expiryYear int, expiryMonth time.Month) (*Card, error) {
	if ownerID == uuid.Nil {
		return nil, fmt.Errorf("card owner id cannot be zero")
	}
	if name == "" {
		return nil, fmt.Errorf("card name cannot be empty")
	}
	if cardType != Debit && cardType != Credit {
		return nil, fmt.Errorf("card type must be %q or %q, got %q", Debit, Credit, cardType)
	}
	if expiryMonth < time.January || expiryMonth > time.December {
		return nil, fmt.Errorf("card expiry month must be within 1..12, got %d", int(expiryMonth))
	}

	return &Card{
		id:          uuid.New(),
		ownerID:     ownerID,
		name:        name,
		cardType:    cardType,
		expiryYear:  expiryYear,
		expiryMonth: expiryMonth,
	}, nil
}

func (c *Card) ID() uuid.UUID {
	return c.id
}

func (c *Card) OwnerID() uuid.UUID {
	return c.ownerID
}

func (c *Card) Name() string {
	return c.name
}

func (c *Card) CardType() CardType {
	return c.cardType
}

func (c *Card) Expiry() (int, time.Month) {
	return c.expiryYear, c.expiryMonth
}

func (c *Card) RemovedAt() *time.Time {
	return c.removedAt
}

// ExpiredAt reports whether the Card has expired at the given moment.
// Expiry is month-granular: a card expiring 2027-08 is valid through the
// last moment of August 2027 and expired from the first moment of September.
// Only calendar year and month are compared.
func (c *Card) ExpiredAt(now time.Time) bool {
	if now.Year() > c.expiryYear {
		return true
	}
	if now.Year() == c.expiryYear && now.Month() > c.expiryMonth {
		return true
	}
	return false
}

// IsActiveAt is a pure derived query: the card is active when it has not
// been removed and has not expired.
func (c *Card) IsActiveAt(now time.Time) bool {
	return c.removedAt == nil && !c.ExpiredAt(now)
}

// Remove records the timestamped statement that the Card is retired.
// It is idempotent: the removal timestamp is only set while unset, so the
// earliest timestamp wins.
func (c *Card) Remove(at time.Time) {
	if c.removedAt == nil {
		c.removedAt = &at
	}
}
