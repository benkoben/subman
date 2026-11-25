package payment

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// cardDate builds a fixed calendar moment at mid-day UTC so removal
// timestamps survive the roundtrip unambiguously.
func cardDate(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

// TestCardBSONRoundtrip checks that the driver, honoring MarshalBSON and
// UnmarshalBSON, maps a Card to its persisted document and back without
// loss. The decode target is a *Card variable, mirroring how the generic
// repository decodes pointer-based aggregates.
func TestCardBSONRoundtrip(t *testing.T) {
	ownerID := uuid.New()

	tests := []struct {
		name      string
		cardType  CardType
		removedAt *time.Time
	}{
		{
			name:     "minimal active card",
			cardType: Debit,
		},
		{
			name:     "credit card without removal",
			cardType: Credit,
		},
		{
			name:      "removed card",
			cardType:  Debit,
			removedAt: cardDatePtr(cardDate(2026, time.May, 1)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want, err := NewCard(ownerID, "Everyday", tt.cardType, 2027, time.August)
			if err != nil {
				t.Fatalf("NewCard() error = %v", err)
			}
			if tt.removedAt != nil {
				want.Remove(*tt.removedAt)
			}

			data, err := bson.Marshal(want)
			if err != nil {
				t.Fatalf("bson.Marshal() error = %v", err)
			}

			var got *Card
			if err := bson.Unmarshal(data, &got); err != nil {
				t.Fatalf("bson.Unmarshal() error = %v", err)
			}

			if diff := cmp.Diff(want, got, cmp.AllowUnexported(Card{})); diff != "" {
				t.Errorf("roundtrip mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func cardDatePtr(t time.Time) *time.Time { return &t }
