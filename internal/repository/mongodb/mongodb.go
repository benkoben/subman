package mongodb

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"subman/internal/repository"
)

// Repository is the MongoDB implementation of repository.Repository. The
// driver marshals and unmarshals the entity itself, so T must own a BSON
// mapping (MarshalBSON/UnmarshalBSON); that keeps the repository generic
// with no reflection on the domain.
//
// K is the type of the persisted identifier: idOf must return exactly the
// value the entity's mapping stores as its document _id, because both the
// Save filter and the FindByID/Delete lookups match on _id.
type Repository[K comparable, T any] struct {
	coll      *mongo.Collection
	idOf      func(T) K
	newEntity func() *T
}

// NewRepository wires a Repository over the given collection. idOf extracts
// the entity's persisted _id; newEntity allocates a zero entity for the
// driver to decode documents into.
func NewRepository[K comparable, T any](coll *mongo.Collection, idOf func(T) K, newEntity func() *T) *Repository[K, T] {
	return &Repository[K, T]{
		coll:      coll,
		idOf:      idOf,
		newEntity: newEntity,
	}
}

// Save upserts the entity: ReplaceOne filtered on the entity's _id, with
// the entity itself as the replacement document.
func (r *Repository[K, T]) Save(ctx context.Context, entity T) error {
	_, err := r.coll.ReplaceOne(ctx,
		bson.M{"_id": r.idOf(entity)},
		entity,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("mongodb repository replace one: %w", err)
	}

	return nil
}

// FindByID returns the entity stored under the given _id, or
// repository.ErrNotFound when the driver reports no documents.
func (r *Repository[K, T]) FindByID(ctx context.Context, id K) (T, error) {
	var zero T

	entity := r.newEntity()
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(entity)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return zero, repository.ErrNotFound
		}
		return zero, fmt.Errorf("mongodb repository find one: %w", err)
	}

	return *entity, nil
}

// FindAll decodes every document in the collection. The order is
// unspecified.
func (r *Repository[K, T]) FindAll(ctx context.Context) ([]T, error) {
	cursor, err := r.coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("mongodb repository find: %w", err)
	}
	defer cursor.Close(ctx)

	out := []T{}
	for cursor.Next(ctx) {
		entity := r.newEntity()
		if err := cursor.Decode(entity); err != nil {
			return nil, fmt.Errorf("mongodb repository decode: %w", err)
		}
		out = append(out, *entity)
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("mongodb repository cursor: %w", err)
	}

	return out, nil
}

// Delete removes the document with the given _id, or returns
// repository.ErrNotFound when the deletion matched nothing.
func (r *Repository[K, T]) Delete(ctx context.Context, id K) error {
	res, err := r.coll.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("mongodb repository delete one: %w", err)
	}
	if res.DeletedCount == 0 {
		return repository.ErrNotFound
	}

	return nil
}

// Connect creates a client for uri and verifies it can reach the
// deployment with a ping before returning.
func Connect(ctx context.Context, uri string) (*mongo.Client, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongodb connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb ping: %w", err)
	}

	return client, nil
}

// Collection returns the named collection in the given database.
func Collection(client *mongo.Client, db, name string) *mongo.Collection {
	return client.Database(db).Collection(name)
}
