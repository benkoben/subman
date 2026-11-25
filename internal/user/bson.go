package user

import (
	"fmt"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// userDoc is the persisted shape of a User. User's fields are unexported,
// so the package owns its own mapping; UUIDs are stored as strings.
type userDoc struct {
	ID    string `bson:"_id"`
	Name  string `bson:"name"`
	Email string `bson:"email"`
}

// MarshalBSON encodes the User as its persisted document. The value
// receiver lets the driver marshal both User and *User values.
func (u User) MarshalBSON() ([]byte, error) {
	return bson.Marshal(userDoc{
		ID:    u.id.String(),
		Name:  u.name,
		Email: u.email,
	})
}

// UnmarshalBSON decodes a persisted document back into the User.
func (u *User) UnmarshalBSON(data []byte) error {
	var d userDoc
	if err := bson.Unmarshal(data, &d); err != nil {
		return err
	}

	id, err := uuid.Parse(d.ID)
	if err != nil {
		return fmt.Errorf("user id %q is not a UUID: %w", d.ID, err)
	}

	u.id = id
	u.name = d.Name
	u.email = d.Email

	return nil
}
