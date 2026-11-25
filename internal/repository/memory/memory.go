package memory

import (
	"context"
	"sync"

	"subman/internal/repository"
)

// Repository is an in-memory repository.Repository backed by a map guarded
// by an RWMutex. It exists to prove the generic contract and to enable
// tests without a server; it is not a production store.
type Repository[K comparable, T any] struct {
	mu   sync.RWMutex
	data map[K]T
	idOf func(T) K
}

// NewRepository constructs an empty in-memory repository. idOf extracts the
// key under which entities are stored, so Save can upsert and FindByID and
// Delete can address entities by ID.
func NewRepository[K comparable, T any](idOf func(T) K) *Repository[K, T] {
	return &Repository[K, T]{
		data: make(map[K]T),
		idOf: idOf,
	}
}

// Save upserts: the entity overwrites any prior entity with the same ID.
func (r *Repository[K, T]) Save(_ context.Context, entity T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[r.idOf(entity)] = entity

	return nil
}

// FindByID returns the stored entity with the given ID, or
// repository.ErrNotFound when it is absent.
func (r *Repository[K, T]) FindByID(_ context.Context, id K) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entity, ok := r.data[id]
	if !ok {
		var zero T
		return zero, repository.ErrNotFound
	}

	return entity, nil
}

// FindAll returns every stored entity. The order is unspecified.
func (r *Repository[K, T]) FindAll(_ context.Context) ([]T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]T, 0, len(r.data))
	for _, entity := range r.data {
		out = append(out, entity)
	}

	return out, nil
}

// Delete removes the entity with the given ID, or returns
// repository.ErrNotFound when it is absent.
func (r *Repository[K, T]) Delete(_ context.Context, id K) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.data[id]; !ok {
		return repository.ErrNotFound
	}

	delete(r.data, id)

	return nil
}
