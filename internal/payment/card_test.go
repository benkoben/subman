package payment

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewCard(t *testing.T) {
	ownerID := uuid.New()
	tests := []struct {
		name        string
		ownerID     uuid.UUID
		cardName    string
		cardType    CardType
		expiryYear  int
		expiryMonth time.Month
		wantErr     bool
	}{
		{
			name:        "valid debit card",
			ownerID:     ownerID,
			cardName:    "Everyday debit",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			wantErr:     false,
		},
		{
			name:        "valid credit card",
			ownerID:     ownerID,
			cardName:    "Travel credit",
			cardType:    Credit,
			expiryYear:  2030,
			expiryMonth: time.December,
			wantErr:     false,
		},
		{
			name:        "zero owner id",
			ownerID:     uuid.Nil,
			cardName:    "Everyday debit",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			wantErr:     true,
		},
		{
			name:        "empty name",
			ownerID:     ownerID,
			cardName:    "",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			wantErr:     true,
		},
		{
			name:        "unknown card type",
			ownerID:     ownerID,
			cardName:    "Everyday debit",
			cardType:    CardType("prepaid"),
			expiryYear:  2027,
			expiryMonth: time.August,
			wantErr:     true,
		},
		{
			name:        "expiry month below one",
			ownerID:     ownerID,
			cardName:    "Everyday debit",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.Month(0),
			wantErr:     true,
		},
		{
			name:        "expiry month above twelve",
			ownerID:     ownerID,
			cardName:    "Everyday debit",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.Month(13),
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCard(tt.ownerID, tt.cardName, tt.cardType, tt.expiryYear, tt.expiryMonth)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewCard() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.ID() == uuid.Nil {
				t.Errorf("ID() = zero uuid, want a generated id")
			}
			if got.OwnerID() != tt.ownerID {
				t.Errorf("OwnerID() = %v, want %v", got.OwnerID(), tt.ownerID)
			}
			if got.Name() != tt.cardName {
				t.Errorf("Name() = %q, want %q", got.Name(), tt.cardName)
			}
			if got.CardType() != tt.cardType {
				t.Errorf("CardType() = %q, want %q", got.CardType(), tt.cardType)
			}
			if year, month := got.Expiry(); year != tt.expiryYear || month != tt.expiryMonth {
				t.Errorf("Expiry() = (%d, %v), want (%d, %v)", year, month, tt.expiryYear, tt.expiryMonth)
			}
			if got.RemovedAt() != nil {
				t.Errorf("RemovedAt() = %v, want nil for a new card", got.RemovedAt())
			}
		})
	}
}

func Test_card_IsActiveAt(t *testing.T) {
	tests := []struct {
		name        string
		cardType    CardType
		expiryYear  int
		expiryMonth time.Month
		remove      bool
		now         time.Time
		want        bool
	}{
		{
			name:        "active card",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      false,
			now:         time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC),
			want:        true,
		},
		{
			name:        "expired card, now inside the month after the expiry month",
			cardType:    Credit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      false,
			now:         time.Date(2027, time.September, 15, 12, 0, 0, 0, time.UTC),
			want:        false,
		},
		{
			name:        "still active on the very last moment of the expiry month",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      false,
			now:         time.Date(2027, time.August, 31, 23, 59, 59, 0, time.UTC),
			want:        true,
		},
		{
			name:        "expired on the first moment of the month after the expiry month",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      false,
			now:         time.Date(2027, time.September, 1, 0, 0, 0, 0, time.UTC),
			want:        false,
		},
		{
			name:        "expired on the first moment of the year after the expiry year",
			cardType:    Credit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      false,
			now:         time.Date(2028, time.January, 1, 0, 0, 0, 0, time.UTC),
			want:        false,
		},
		{
			name:        "removed card inactive despite a future expiry",
			cardType:    Debit,
			expiryYear:  2027,
			expiryMonth: time.August,
			remove:      true,
			now:         time.Date(2026, time.February, 15, 12, 0, 0, 0, time.UTC),
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewCard(uuid.New(), "Test card", tt.cardType, tt.expiryYear, tt.expiryMonth)
			if err != nil {
				t.Fatalf("NewCard() unexpected error: %v", err)
			}
			if tt.remove {
				c.Remove(time.Date(2025, time.January, 1, 12, 0, 0, 0, time.UTC))
			}
			if got := c.IsActiveAt(tt.now); got != tt.want {
				t.Errorf("IsActiveAt(%v) = %v, want %v", tt.now, got, tt.want)
			}
		})
	}
}

func Test_card_Remove(t *testing.T) {
	c, err := NewCard(uuid.New(), "Everyday debit", Debit, 2027, time.August)
	if err != nil {
		t.Fatalf("NewCard() unexpected error: %v", err)
	}

	if c.RemovedAt() != nil {
		t.Fatalf("RemovedAt() = %v, want nil before Remove", c.RemovedAt())
	}

	first := time.Date(2025, time.March, 10, 12, 0, 0, 0, time.UTC)
	c.Remove(first)
	if c.RemovedAt() == nil || !c.RemovedAt().Equal(first) {
		t.Fatalf("RemovedAt() = %v, want %v", c.RemovedAt(), first)
	}

	// Remove is idempotent and keeps the earliest timestamp.
	later := time.Date(2025, time.March, 20, 12, 0, 0, 0, time.UTC)
	c.Remove(later)
	if c.RemovedAt() == nil || !c.RemovedAt().Equal(first) {
		t.Errorf("RemovedAt() = %v, want %v kept as the earliest timestamp", c.RemovedAt(), first)
	}
}
