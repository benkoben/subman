package payment

import (
	"fmt"
	"time"
)

type cardType string

const (
	Debid  cardType = "debid"
	Credit cardType = "credit"
)

type cardStatus string

const (
	StatusExpired   cardStatus = "expired"
	StatusActive    cardStatus = "active"
	StatusCancelled cardStatus = "cancelled"
)

type card struct {
	name           string
	cardType       cardType
	expirationDate time.Time
	status         cardStatus
}

func (c *card) IsActive() bool {
	c.expireIfDue()
	fmt.Println(c.status)
	return c.status == StatusActive
}

// Cancel sets StatusCancelled on card. No-op if card.status already is StatusCancelled
func (c *card) Cancel() {
	if c.status == StatusActive {
		c.status = StatusCancelled
	}
}

// expireIfDue sets StatusCancelled on card if expirationDate has passed
// no-op if the card has yet to expire or the card is cancelled.
func (c *card) expireIfDue() {
	if c.status == StatusCancelled {
		return
	}

	if time.Now().After(c.expirationDate) {
		c.status = StatusExpired
	}
}

// Factory to create card
func NewCard(name string, cardType cardType, expirationDate time.Time) *card {
	return &card{
		name:           name,
		cardType:       cardType,
		expirationDate: expirationDate,
		status:         StatusActive,
	}
}
