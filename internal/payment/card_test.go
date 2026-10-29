package payment

import (
	"testing"
	"time"
)

func Test_card_IsActive(t *testing.T) {
	tests := []struct {
		name           string
		cardType       cardType
		expirationDate time.Time
		cardFunc       func(*card)
		want           bool
	}{
		{
			name:           "active card",
			cardType:       Debid,
			expirationDate: time.Now().Add(time.Hour * 24 * 365),
			cardFunc:       nil,
			want:           true,
		},
		{
			name:           "expired card",
			cardType:       Debid,
			expirationDate: time.Now().Add(-time.Hour * 24 * 365),
			cardFunc:       nil,
			want:           false,
		},
		{
			name:           "cancelled card",
			cardType:       Debid,
			expirationDate: time.Now().Add(time.Hour * 24 * 365),
			cardFunc: func(c *card) {
				c.status = StatusCancelled
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCard(tt.name, tt.cardType, tt.expirationDate)
			if tt.cardFunc != nil {
				tt.cardFunc(c)
			}
			got := c.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
