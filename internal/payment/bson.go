package payment

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// cardDoc is the persisted shape of a Card. Card's fields are unexported,
// so the package owns its own mapping; UUIDs are stored as strings, the
// expiry date keeps its year and month, and removedAt is nullable.
type cardDoc struct {
	ID          string     `bson:"_id"`
	OwnerID     string     `bson:"ownerID"`
	Name        string     `bson:"name"`
	CardType    string     `bson:"cardType"`
	ExpiryYear  int        `bson:"expiryYear"`
	ExpiryMonth int        `bson:"expiryMonth"`
	RemovedAt   *time.Time `bson:"removedAt,omitempty"`
}

// MarshalBSON encodes the Card as its persisted document. The value
// receiver lets the driver marshal both Card and *Card values.
func (c Card) MarshalBSON() ([]byte, error) {
	return bson.Marshal(cardDoc{
		ID:          c.id.String(),
		OwnerID:     c.ownerID.String(),
		Name:        c.name,
		CardType:    string(c.cardType),
		ExpiryYear:  c.expiryYear,
		ExpiryMonth: int(c.expiryMonth),
		RemovedAt:   c.removedAt,
	})
}

// UnmarshalBSON decodes a persisted document back into the Card.
func (c *Card) UnmarshalBSON(data []byte) error {
	var d cardDoc
	if err := bson.Unmarshal(data, &d); err != nil {
		return err
	}

	id, err := uuid.Parse(d.ID)
	if err != nil {
		return fmt.Errorf("card id %q is not a UUID: %w", d.ID, err)
	}
	ownerID, err := uuid.Parse(d.OwnerID)
	if err != nil {
		return fmt.Errorf("card owner id %q is not a UUID: %w", d.OwnerID, err)
	}
	if d.ExpiryMonth < int(time.January) || d.ExpiryMonth > int(time.December) {
		return fmt.Errorf("card expiry month must be within 1..12, got %d", d.ExpiryMonth)
	}

	c.id = id
	c.ownerID = ownerID
	c.name = d.Name
	c.cardType = CardType(d.CardType)
	c.expiryYear = d.ExpiryYear
	c.expiryMonth = time.Month(d.ExpiryMonth)
	c.removedAt = d.RemovedAt

	return nil
}
