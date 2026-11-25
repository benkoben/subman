package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"subman/internal/repository"
)

// testEntity is a minimal fixture entity for exercising the generic
// contract without pulling in a domain aggregate.
type testEntity struct {
	id   string
	name string
}

func idOf(e testEntity) string { return e.id }

// intEntity proves the contract is generic over the key type as well.
type intEntity struct {
	id   int
	name string
}

func intIDOf(e intEntity) int { return e.id }

var _ repository.Repository[string, testEntity] = NewRepository(idOf)

func TestRepositorySaveAndFindByID(t *testing.T) {
	tests := []struct {
		name   string
		entity testEntity
	}{
		{
			name:   "first entity",
			entity: testEntity{id: "alpha", name: "Alpha"},
		},
		{
			name:   "second entity",
			entity: testEntity{id: "beta", name: "Beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewRepository(idOf)

			if err := repo.Save(context.Background(), tt.entity); err != nil {
				t.Fatalf("Save() error = %v", err)
			}

			got, err := repo.FindByID(context.Background(), tt.entity.id)
			if err != nil {
				t.Fatalf("FindByID() error = %v", err)
			}
			if diff := cmp.Diff(tt.entity, got); diff != "" {
				t.Errorf("FindByID() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRepositorySaveUpsertOverwrites(t *testing.T) {
	repo := NewRepository(idOf)

	original := testEntity{id: "alpha", name: "Alpha"}
	updated := testEntity{id: "alpha", name: "Alpha Prime"}

	if err := repo.Save(context.Background(), original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := repo.Save(context.Background(), updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if diff := cmp.Diff(updated, got); diff != "" {
		t.Errorf("FindByID() after upsert mismatch (-want +got):\n%s", diff)
	}

	all, err := repo.FindAll(context.Background())
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(all) != 1 {
		t.Errorf("FindAll() after upsert returned %d entities, want 1", len(all))
	}
}

func TestRepositoryFindByIDNotFound(t *testing.T) {
	repo := NewRepository(idOf)
	if err := repo.Save(context.Background(), testEntity{id: "alpha", name: "Alpha"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	tests := []struct {
		name string
		id   string
	}{
		{name: "never stored id", id: "missing"},
		{name: "stored then deleted id", id: "removed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.id == "removed" {
				if err := repo.Delete(context.Background(), tt.id); err != nil {
					t.Fatalf("Delete() error = %v", err)
				}
			}

			got, err := repo.FindByID(context.Background(), tt.id)
			if !errors.Is(err, repository.ErrNotFound) {
				t.Fatalf("FindByID() error = %v, want repository.ErrNotFound", err)
			}
			if got != (testEntity{}) {
				t.Errorf("FindByID() on miss returned %+v, want zero value", got)
			}
		})
	}
}

func TestRepositoryFindAll(t *testing.T) {
	entities := []testEntity{
		{id: "alpha", name: "Alpha"},
		{id: "beta", name: "Beta"},
		{id: "gamma", name: "Gamma"},
	}

	t.Run("empty repository", func(t *testing.T) {
		repo := NewRepository(idOf)

		all, err := repo.FindAll(context.Background())
		if err != nil {
			t.Fatalf("FindAll() error = %v", err)
		}
		if len(all) != 0 {
			t.Errorf("FindAll() on empty repository returned %d entities, want 0", len(all))
		}
	})

	t.Run("all stored entities are returned", func(t *testing.T) {
		repo := NewRepository(idOf)
		for _, e := range entities {
			if err := repo.Save(context.Background(), e); err != nil {
				t.Fatalf("Save() error = %v", err)
			}
		}

		all, err := repo.FindAll(context.Background())
		if err != nil {
			t.Fatalf("FindAll() error = %v", err)
		}
		// FindAll's order is unspecified, so compare as a set keyed by ID.
		byID := make(map[string]testEntity, len(all))
		for _, e := range all {
			byID[e.id] = e
		}
		if diff := cmp.Diff(map[string]testEntity{
			"alpha": entities[0],
			"beta":  entities[1],
			"gamma": entities[2],
		}, byID); diff != "" {
			t.Errorf("FindAll() mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestRepositoryDelete(t *testing.T) {
	repo := NewRepository(idOf)
	if err := repo.Save(context.Background(), testEntity{id: "alpha", name: "Alpha"}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := repo.Delete(context.Background(), "alpha"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := repo.FindByID(context.Background(), "alpha"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("FindByID() after delete error = %v, want repository.ErrNotFound", err)
	}

	if err := repo.Delete(context.Background(), "alpha"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("Delete() of absent id error = %v, want repository.ErrNotFound", err)
	}
}

func TestRepositoryGenericOverKeyType(t *testing.T) {
	repo := NewRepository(intIDOf)
	entity := intEntity{id: 7, name: "Seven"}

	if err := repo.Save(context.Background(), entity); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(context.Background(), 7)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got != entity {
		t.Errorf("FindByID() = %+v, want %+v", got, entity)
	}
}
