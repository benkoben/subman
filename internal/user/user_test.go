package user

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name    string
		userName string
		email   string
		wantErr bool
	}{
		{
			name:    "valid user",
			userName: "Erika Lindqvist",
			email:   "erika@example.com",
			wantErr: false,
		},
		{
			name:    "empty name",
			userName: "",
			email:   "erika@example.com",
			wantErr: true,
		},
		{
			name:    "email missing at sign",
			userName: "Erika Lindqvist",
			email:   "erika.example.com",
			wantErr: true,
		},
		{
			name:    "empty email",
			userName: "Erika Lindqvist",
			email:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUser(tt.userName, tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewUser() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.ID() == uuid.Nil {
				t.Errorf("ID() = zero uuid, want a generated id")
			}
			if got.Name() != tt.userName {
				t.Errorf("Name() = %q, want %q", got.Name(), tt.userName)
			}
			if got.Email() != tt.email {
				t.Errorf("Email() = %q, want %q", got.Email(), tt.email)
			}
		})
	}
}
