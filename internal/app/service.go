package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"subman/internal/payment"
	"subman/internal/plan"
	"subman/internal/repository"
	"subman/internal/subscription"
	"subman/internal/user"
)

// ErrInvalidInput marks errors caused by input the domain rejects.
var ErrInvalidInput = errors.New("invalid input")

// Service coordinates the use cases of subman over its repositories.
type Service struct {
	users         repository.Repository[string, user.User]
	cards         repository.Repository[string, *payment.Card]
	subscriptions repository.Repository[string, *subscription.Subscription]
}

// NewService constructs a Service over the given repositories.
func NewService(
	users repository.Repository[string, user.User],
	cards repository.Repository[string, *payment.Card],
	subscriptions repository.Repository[string, *subscription.Subscription],
) *Service {
	return &Service{
		users:         users,
		cards:         cards,
		subscriptions: subscriptions,
	}
}

// CreateUser validates and constructs a User, then saves it.
func (s *Service) CreateUser(ctx context.Context, name, email string) (user.User, error) {
	u, err := user.NewUser(name, email)
	if err != nil {
		return user.User{}, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	if err := s.users.Save(ctx, u); err != nil {
		return user.User{}, err
	}

	return u, nil
}

// ListUsers returns every stored User. The order is unspecified.
func (s *Service) ListUsers(ctx context.Context) ([]user.User, error) {
	return s.users.FindAll(ctx)
}

// CreateCard validates and constructs a Card in the wallet of the given
// User, then saves it. It returns repository.ErrNotFound when the User is
// absent.
func (s *Service) CreateCard(ctx context.Context, ownerID uuid.UUID, name string, cardType payment.CardType, expiryYear int, expiryMonth time.Month) (*payment.Card, error) {
	if _, err := s.users.FindByID(ctx, ownerID.String()); err != nil {
		return nil, err
	}

	c, err := payment.NewCard(ownerID, name, cardType, expiryYear, expiryMonth)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	if err := s.cards.Save(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// ListCards returns the Cards in the wallet of the given User. The order is
// unspecified. It returns repository.ErrNotFound when the User is absent.
func (s *Service) ListCards(ctx context.Context, ownerID uuid.UUID) ([]*payment.Card, error) {
	if _, err := s.users.FindByID(ctx, ownerID.String()); err != nil {
		return nil, err
	}

	all, err := s.cards.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	owned := make([]*payment.Card, 0, len(all))
	for _, c := range all {
		if c.OwnerID() == ownerID {
			owned = append(owned, c)
		}
	}

	return owned, nil
}

// Profile is the view of a User's Subscriptions and Wallet. It is composed
// on demand and never stored.
type Profile struct {
	User          user.User
	Cards         []*payment.Card
	Subscriptions []*subscription.Subscription
}

// Profile retrieves a User together with the Cards and Subscriptions it
// owns. It returns repository.ErrNotFound when the User is absent.
func (s *Service) Profile(ctx context.Context, ownerID uuid.UUID) (Profile, error) {
	u, err := s.users.FindByID(ctx, ownerID.String())
	if err != nil {
		return Profile{}, err
	}

	cards, err := s.ListCards(ctx, ownerID)
	if err != nil {
		return Profile{}, err
	}

	subs, err := s.ListSubscriptions(ctx, ownerID)
	if err != nil {
		return Profile{}, err
	}

	return Profile{User: u, Cards: cards, Subscriptions: subs}, nil
}

// SubscriptionInput carries the user-entered fields of a Subscription to be
// recorded.
type SubscriptionInput struct {
	PlanName         string
	PlanVendor       string
	PlanDescription  *string
	BillingFrequency plan.BillingFrequency
	InitialAmount    int64
	StartDate        time.Time
	TrialDays        *int
	PaymentMethodID  *uuid.UUID
}

// CreateSubscription validates and constructs a Subscription for the given
// User, then saves it. It returns repository.ErrNotFound when the User is
// absent, or when the referenced PaymentMethod is absent or owned by another
// User.
func (s *Service) CreateSubscription(ctx context.Context, ownerID uuid.UUID, in SubscriptionInput) (*subscription.Subscription, error) {
	if err := s.requireUser(ctx, ownerID); err != nil {
		return nil, err
	}
	if in.PaymentMethodID != nil {
		if *in.PaymentMethodID == uuid.Nil {
			return nil, fmt.Errorf("%w: payment method id is zero", ErrInvalidInput)
		}
		if err := s.requireOwnedCard(ctx, ownerID, *in.PaymentMethodID); err != nil {
			return nil, err
		}
	}

	p, err := plan.NewPlan(in.PlanName, in.PlanVendor, in.PlanDescription, in.BillingFrequency)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	opts := []subscription.Option{subscription.WithStartDate(in.StartDate)}
	if in.TrialDays != nil {
		opts = append(opts, subscription.WithTrial(*in.TrialDays))
	}
	if in.PaymentMethodID != nil {
		opts = append(opts, subscription.WithPaymentMethodID(*in.PaymentMethodID))
	}

	sub, err := subscription.NewSubscription(ownerID, p, in.InitialAmount, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	if err := s.subscriptions.Save(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// ListSubscriptions returns the Subscriptions of the given User. The order is
// unspecified. It returns repository.ErrNotFound when the User is absent.
func (s *Service) ListSubscriptions(ctx context.Context, ownerID uuid.UUID) ([]*subscription.Subscription, error) {
	if err := s.requireUser(ctx, ownerID); err != nil {
		return nil, err
	}

	all, err := s.subscriptions.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	owned := make([]*subscription.Subscription, 0, len(all))
	for _, sub := range all {
		if sub.UserID() == ownerID {
			owned = append(owned, sub)
		}
	}

	return owned, nil
}

// RemoveCard records the timestamped Removal of a Card in the wallet of the
// given User, then saves it. A Card owned by another User is reported as
// repository.ErrNotFound. Removal is idempotent: the earliest timestamp wins.
func (s *Service) RemoveCard(ctx context.Context, ownerID, cardID uuid.UUID, at time.Time) (*payment.Card, error) {
	if _, err := s.users.FindByID(ctx, ownerID.String()); err != nil {
		return nil, err
	}

	c, err := s.cards.FindByID(ctx, cardID.String())
	if err != nil {
		return nil, err
	}
	if c.OwnerID() != ownerID {
		return nil, repository.ErrNotFound
	}

	c.Remove(at)

	if err := s.cards.Save(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// CancelSubscription records the Cancellation of a Subscription owned by the
// given User, then saves it. Cancellation is idempotent: the earliest
// timestamp wins. A timestamp before the start date is reported as
// ErrInvalidInput and nothing is saved. It returns repository.ErrNotFound
// when the User or Subscription is absent, or when the Subscription is owned
// by another User.
func (s *Service) CancelSubscription(ctx context.Context, ownerID, subscriptionID uuid.UUID, at time.Time) (*subscription.Subscription, error) {
	sub, err := s.requireOwnedSubscription(ctx, ownerID, subscriptionID)
	if err != nil {
		return nil, err
	}

	if !sub.CancelAt(at) && sub.CancelledAt() == nil {
		return nil, fmt.Errorf("%w: cancellation at %s is before the start date %s",
			ErrInvalidInput, at.Format(time.RFC3339), sub.StartDate().Format(time.RFC3339))
	}

	if err := s.subscriptions.Save(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// AddPriceInterval records a Price Interval in the pricing history of a
// Subscription owned by the given User, then saves it. Domain rejections,
// such as a non-positive amount, a From before the first charge, or a
// duplicate From, are reported as ErrInvalidInput. It returns
// repository.ErrNotFound when the User or Subscription is absent, or when the
// Subscription is owned by another User.
func (s *Service) AddPriceInterval(ctx context.Context, ownerID, subscriptionID uuid.UUID, from time.Time, amount int64) (*subscription.Subscription, error) {
	sub, err := s.requireOwnedSubscription(ctx, ownerID, subscriptionID)
	if err != nil {
		return nil, err
	}

	if err := sub.AddPriceInterval(from, amount); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}

	if err := s.subscriptions.Save(ctx, sub); err != nil {
		return nil, err
	}

	return sub, nil
}

// requireUser returns repository.ErrNotFound when no User has the given ID.
func (s *Service) requireUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.users.FindByID(ctx, userID.String())
	return err
}

// requireOwnedCard returns repository.ErrNotFound when no Card has the given
// ID or when the Card is owned by another User.
func (s *Service) requireOwnedCard(ctx context.Context, ownerID, cardID uuid.UUID) error {
	c, err := s.cards.FindByID(ctx, cardID.String())
	if err != nil {
		return err
	}
	if c.OwnerID() != ownerID {
		return repository.ErrNotFound
	}
	return nil
}

// requireOwnedSubscription returns the Subscription with the given ID when it
// is owned by the given User. It returns repository.ErrNotFound when the User
// or Subscription is absent, or when the Subscription is owned by another
// User.
func (s *Service) requireOwnedSubscription(ctx context.Context, ownerID, subscriptionID uuid.UUID) (*subscription.Subscription, error) {
	if err := s.requireUser(ctx, ownerID); err != nil {
		return nil, err
	}

	sub, err := s.subscriptions.FindByID(ctx, subscriptionID.String())
	if err != nil {
		return nil, err
	}
	if sub.UserID() != ownerID {
		return nil, repository.ErrNotFound
	}
	return sub, nil
}
