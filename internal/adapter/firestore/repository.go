// Package firestore stores load_config documents in a named Firestore database.
package firestore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	gfs "cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// Repository is the Firestore adapter for load_config.
type Repository struct {
	client     *gfs.Client
	collection string
}

// NewRepository opens a client with Application Default Credentials.
// FIRESTORE_EMULATOR_HOST is honored by the client library when it is set.
func NewRepository(ctx context.Context, cfg model.AppConfig) (*Repository, func(), error) {
	client, err := openClient(ctx, cfg.Firestore.ProjectID, cfg.Firestore.DatabaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating firestore client: %w", err)
	}

	repo := &Repository{client: client, collection: cfg.Firestore.Collection}
	cleanup := func() {
		_ = client.Close()
	}

	return repo, cleanup, nil
}

// List reads the whole collection. The console filters in memory because derived
// fields are not stored and the collection is small.
func (r *Repository) List(ctx context.Context) ([]model.LoadConfig, error) {
	docs, err := r.client.Collection(r.collection).Documents(ctx).GetAll()
	if err != nil {
		return nil, fmt.Errorf("listing load configs: %w", err)
	}

	items := make([]model.LoadConfig, 0, len(docs))
	for _, doc := range docs {
		cfg, decodeErr := decode(doc)
		if decodeErr != nil {
			return nil, fmt.Errorf("decoding load config %s: %w", doc.Ref.ID, decodeErr)
		}

		items = append(items, cfg)
	}

	return items, nil
}

// Get reads one document by id.
func (r *Repository) Get(ctx context.Context, id string) (model.LoadConfig, error) {
	doc, err := r.client.Collection(r.collection).Doc(id).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return model.LoadConfig{}, model.ErrNotFound
	}

	if err != nil {
		return model.LoadConfig{}, fmt.Errorf("reading load config: %w", err)
	}

	return decode(doc)
}

// Create fails when the document id already exists.
func (r *Repository) Create(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error) {
	_, err := r.client.Collection(r.collection).Doc(cfg.ID).Create(ctx, documentData(cfg))
	if status.Code(err) == codes.AlreadyExists {
		return model.LoadConfig{}, model.ErrConflict
	}

	if err != nil {
		return model.LoadConfig{}, fmt.Errorf("creating load config: %w", err)
	}

	return r.Get(ctx, cfg.ID)
}

// Move writes cfg at cfg.ID and deletes fromID when the id changed.
// Unknown fields on the source document are copied onto the destination.
func (r *Repository) Move(ctx context.Context, fromID string, cfg model.LoadConfig) (model.LoadConfig, error) {
	if fromID == cfg.ID {
		return r.Update(ctx, cfg)
	}

	oldRef := r.client.Collection(r.collection).Doc(fromID)
	newRef := r.client.Collection(r.collection).Doc(cfg.ID)

	err := r.client.RunTransaction(ctx, func(_ context.Context, tx *gfs.Transaction) error {
		oldSnap, err := tx.Get(oldRef)
		if status.Code(err) == codes.NotFound {
			return model.ErrNotFound
		}

		if err != nil {
			return err
		}

		_, err = tx.Get(newRef)
		switch {
		case err == nil:
			return model.ErrConflict
		case status.Code(err) != codes.NotFound:
			return err
		}

		if err = tx.Set(newRef, mergePreservingUnknown(oldSnap.Data(), documentData(cfg))); err != nil {
			return err
		}

		return tx.Delete(oldRef)
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) || errors.Is(err, model.ErrConflict) {
			return model.LoadConfig{}, err
		}

		if status.Code(err) == codes.NotFound {
			return model.LoadConfig{}, model.ErrNotFound
		}

		return model.LoadConfig{}, fmt.Errorf("moving load config: %w", err)
	}

	return r.Get(ctx, cfg.ID)
}

// Update replaces modeled fields and keeps every document field the struct does not declare.
// Removed mapping columns are dropped. Unknown fields on a mapping that remains, such as
// isPartitionKey, are kept.
func (r *Repository) Update(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error) {
	ref := r.client.Collection(r.collection).Doc(cfg.ID)

	err := r.client.RunTransaction(ctx, func(_ context.Context, tx *gfs.Transaction) error {
		snap, err := tx.Get(ref)
		if status.Code(err) == codes.NotFound {
			return model.ErrNotFound
		}

		if err != nil {
			return err
		}

		return tx.Set(ref, mergePreservingUnknown(snap.Data(), documentData(cfg)))
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) || status.Code(err) == codes.NotFound {
			return model.LoadConfig{}, model.ErrNotFound
		}

		return model.LoadConfig{}, fmt.Errorf("updating load config: %w", err)
	}

	return r.Get(ctx, cfg.ID)
}

// Delete removes a document that exists.
func (r *Repository) Delete(ctx context.Context, id string) error {
	ref := r.client.Collection(r.collection).Doc(id)

	_, err := ref.Get(ctx)
	if status.Code(err) == codes.NotFound {
		return model.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("reading load config: %w", err)
	}

	if _, err = ref.Delete(ctx); err != nil {
		return fmt.Errorf("deleting load config: %w", err)
	}

	return nil
}

func decode(doc *gfs.DocumentSnapshot) (model.LoadConfig, error) {
	var cfg model.LoadConfig
	if err := doc.DataTo(&cfg); err != nil {
		return model.LoadConfig{}, err
	}

	cfg.ID = doc.Ref.ID
	cfg.CreateTime = doc.CreateTime

	cfg.UpdateTime = doc.UpdateTime
	if cfg.Mappings == nil {
		cfg.Mappings = map[string]model.Mapping{}
	}

	return cfg, nil
}

func openClient(ctx context.Context, projectID, databaseID string) (*gfs.Client, error) {
	attempts := 1
	if os.Getenv("FIRESTORE_EMULATOR_HOST") != "" {
		attempts = 10
	}

	var (
		client *gfs.Client
		err    error
	)

	for attempt := 1; attempt <= attempts; attempt++ {
		client, err = gfs.NewClientWithDatabase(ctx, projectID, databaseID)
		if err == nil {
			return client, nil
		}

		if attempt < attempts {
			time.Sleep(time.Second)
		}
	}

	return nil, err
}
