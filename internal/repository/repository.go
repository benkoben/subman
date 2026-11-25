package repository

import (
	"context"
	"errors"
)

// Repository is the generic persistence contract for a single entity type.
// T is the entity, K the type of its persisted identifier: implementations
// key documents by that identifier, so idOf must yield the same value the
// entity itself marshals as its document _id.
type Repository[K comparable, T any] interface {
	// Save upserts the entity, keyed by its ID: an existing document with
	// the same ID is overwritten, an absent one is created.
	Save(ctx context.Context, entity T) error

	// FindByID returns the entity with the given ID, or ErrNotFound when
	// no such entity is stored.
	FindByID(ctx context.Context, id K) (T, error)

	// FindAll returns every stored entity. The order is unspecified.
	FindAll(ctx context.Context) ([]T, error)

	// Delete removes the entity with the given ID, or returns ErrNotFound
	// when no such entity is stored.
	Delete(ctx context.Context, id K) error
}

// ErrNotFound is returned by FindByID and Delete when no entity with the
// requested ID is stored.
var ErrNotFound = errors.New("repository: entity not found")
