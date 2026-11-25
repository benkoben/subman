package user

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestUserBSONRoundtrip checks that the driver, honoring MarshalBSON and
// UnmarshalBSON, maps a User to its persisted document and back without
// loss. No server is involved: the raw document bytes are roundtripped.
func TestUserBSONRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		email string
	}{
		{name: "minimal user", email: "ada@example.com"},
		{name: "second user", email: "grace@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := NewUser(tt.name, tt.email)
			if err != nil {
				t.Fatalf("NewUser() error = %v", err)
			}

			data, err := bson.Marshal(want)
			if err != nil {
				t.Fatalf("bson.Marshal() error = %v", err)
			}

			var got User
			if err := bson.Unmarshal(data, &got); err != nil {
				t.Fatalf("bson.Unmarshal() error = %v", err)
			}

			if diff := cmp.Diff(want, got, cmp.AllowUnexported(User{})); diff != "" {
				t.Errorf("roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
