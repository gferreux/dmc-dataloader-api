// Package memory is an in-memory load_config repository used by tests.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// Repository stores load configs in process memory.
type Repository struct {
	mu    sync.Mutex
	items map[string]model.LoadConfig
	now   func() time.Time
}

// NewRepository returns an empty repository.
func NewRepository() *Repository {
	return &Repository{
		items: map[string]model.LoadConfig{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

// List returns every stored document without derived fields.
func (r *Repository) List(_ context.Context) ([]model.LoadConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]model.LoadConfig, 0, len(r.items))
	for _, cfg := range r.items {
		items = append(items, cfg.Clone())
	}

	return items, nil
}

// Get returns one document.
func (r *Repository) Get(_ context.Context, id string) (model.LoadConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, ok := r.items[id]
	if !ok {
		return model.LoadConfig{}, model.ErrNotFound
	}

	return cfg.Clone(), nil
}

// Create inserts a document and stamps metadata timestamps.
func (r *Repository) Create(_ context.Context, cfg model.LoadConfig) (model.LoadConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[cfg.ID]; ok {
		return model.LoadConfig{}, model.ErrConflict
	}

	stored := stamp(cfg, r.now(), r.now())
	r.items[cfg.ID] = stored

	return stored.Clone(), nil
}

// Update replaces a document and refreshes updateTime.
func (r *Repository) Update(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error) {
	return r.Move(ctx, cfg.ID, cfg)
}

// Move stores cfg under its id and drops fromID when the id changed.
func (r *Repository) Move(_ context.Context, fromID string, cfg model.LoadConfig) (model.LoadConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	current, ok := r.items[fromID]
	if !ok {
		return model.LoadConfig{}, model.ErrNotFound
	}

	if fromID != cfg.ID {
		if _, exists := r.items[cfg.ID]; exists {
			return model.LoadConfig{}, model.ErrConflict
		}

		delete(r.items, fromID)
	}

	stored := stamp(cfg, current.CreateTime, r.now())
	r.items[cfg.ID] = stored

	return stored.Clone(), nil
}

// Delete removes a document.
func (r *Repository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[id]; !ok {
		return model.ErrNotFound
	}

	delete(r.items, id)

	return nil
}

func stamp(cfg model.LoadConfig, created, updated time.Time) model.LoadConfig {
	stored := cfg.Clone()
	stored.PartnerType = ""
	stored.ImportType = ""
	stored.Identity = model.Identity{}
	stored.CreateTime = created
	stored.UpdateTime = updated

	return stored
}
