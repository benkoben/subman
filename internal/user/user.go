package user

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// User is the person whose subscriptions and payment methods subman tracks.
// Identity only — name and email; owns a Wallet.
type User struct {
	id    uuid.UUID
	name  string
	email string
}

// NewUser validates and constructs a User. It errors on an empty name or an
// email that does not contain "@".
func NewUser(name, email string) (User, error) {
	var u User

	if name == "" {
		return u, fmt.Errorf("user name cannot be empty")
	}
	if !strings.Contains(email, "@") {
		return u, fmt.Errorf("user email must contain @, got %q", email)
	}

	u.id = uuid.New()
	u.name = name
	u.email = email

	return u, nil
}

func (u *User) ID() uuid.UUID {
	return u.id
}

func (u *User) Name() string {
	return u.name
}

func (u *User) Email() string {
	return u.email
}
