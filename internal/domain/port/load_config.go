// Package port defines the interfaces the usecases depend on.
package port

import (
	"context"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// LoadConfigRepository stores load_config documents.
type LoadConfigRepository interface {
	List(ctx context.Context) ([]model.LoadConfig, error)
	Get(ctx context.Context, id string) (model.LoadConfig, error)
	Create(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error)
	Update(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error)
	// Move writes cfg under cfg.ID and deletes fromID when the document id changes.
	// Unknown document fields from fromID are kept. fromID == cfg.ID updates in place.
	Move(ctx context.Context, fromID string, cfg model.LoadConfig) (model.LoadConfig, error)
	Delete(ctx context.Context, id string) error
}

// OrganizationDirectory reads organizations and advertiser accounts.
// Implementations return ErrDirectoryUnavailable when the source cannot be read.
type OrganizationDirectory interface {
	ListOrganizations(ctx context.Context, kind string) ([]model.NamedRef, error)
	ListAccounts(ctx context.Context, organizationID string) ([]model.NamedRef, error)
}

// LoadConfigUsecase is the console's load_config application service.
type LoadConfigUsecase interface {
	List(ctx context.Context, filter model.ListFilter) ([]model.LoadConfig, error)
	Get(ctx context.Context, id string) (model.LoadConfig, error)
	Create(ctx context.Context, cfg model.LoadConfig) (model.LoadConfig, error)
	Update(ctx context.Context, id string, cfg model.LoadConfig) (model.LoadConfig, error)
	Delete(ctx context.Context, id string) error
	Validate(ctx context.Context, cfg model.LoadConfig) (model.ValidationReport, error)
	TestPattern(ctx context.Context, pattern, path string) (model.PatternTest, error)
	Derive(ctx context.Context, identity model.Identity) (model.DerivedConfig, error)
	ListOrganizations(ctx context.Context, kind string) ([]model.NamedRef, error)
	ListAccounts(ctx context.Context, organizationID string) ([]model.NamedRef, error)
	ListBases(ctx context.Context, orgSlug string) ([]model.NamedRef, error)
	Templates(ctx context.Context) []model.Template
	Meta(ctx context.Context) model.Meta
}

// Principal is the caller authenticated by the pluggable authenticator.
type Principal struct {
	Subject string
	Email   string
}

// Credentials is the material the authenticator needs from an HTTP request.
type Credentials struct {
	IAPJWT string
}

// Authenticator accepts or rejects a caller. Implementations must be safe for concurrent use.
type Authenticator interface {
	Authenticate(ctx context.Context, creds Credentials) (Principal, error)
}
