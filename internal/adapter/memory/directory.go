package memory

import (
	"context"
	"sync"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

type storedOrg struct {
	id          string
	name        string
	kind        string
	deactivated bool
}

type storedAccount struct {
	id             string
	name           string
	organizationID string
	deactivated    bool
}

// Directory is an in-memory organization source used by tests.
type Directory struct {
	mu        sync.Mutex
	orgs      []storedOrg
	accounts  []storedAccount
	readError error
}

// NewDirectory returns an empty directory that answers every read.
func NewDirectory() *Directory {
	return &Directory{}
}

// AddOrganization stores an active organization.
func (d *Directory) AddOrganization(id, name, kind string) {
	d.addOrganization(id, name, kind, false)
}

// AddDeactivatedOrganization stores an organization that resolution must skip.
func (d *Directory) AddDeactivatedOrganization(id, name, kind string) {
	d.addOrganization(id, name, kind, true)
}

func (d *Directory) addOrganization(id, name, kind string, deactivated bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.orgs = append(d.orgs, storedOrg{id: id, name: name, kind: kind, deactivated: deactivated})
}

// AddAccount stores an active advertiser account.
func (d *Directory) AddAccount(id, name, organizationID string) {
	d.addAccount(id, name, organizationID, false)
}

// AddDeactivatedAccount stores an account that resolution must skip.
func (d *Directory) AddDeactivatedAccount(id, name, organizationID string) {
	d.addAccount(id, name, organizationID, true)
}

func (d *Directory) addAccount(id, name, organizationID string, deactivated bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.accounts = append(d.accounts, storedAccount{
		id:             id,
		name:           name,
		organizationID: organizationID,
		deactivated:    deactivated,
	})
}

// SetUnavailable makes later reads fail like a denied or unreachable Firestore source.
func (d *Directory) SetUnavailable() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.readError = model.ErrDirectoryUnavailable
}

// ListOrganizations returns active organizations of one kind.
func (d *Directory) ListOrganizations(_ context.Context, kind string) ([]model.NamedRef, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.readError != nil {
		return nil, d.readError
	}

	refs := make([]model.NamedRef, 0)

	for _, org := range d.orgs {
		if org.deactivated || org.kind != kind {
			continue
		}

		refs = append(refs, model.NamedRef{ID: org.id, Name: org.name})
	}

	return refs, nil
}

// ListAccounts returns active accounts of one organization.
func (d *Directory) ListAccounts(_ context.Context, organizationID string) ([]model.NamedRef, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.readError != nil {
		return nil, d.readError
	}

	refs := make([]model.NamedRef, 0)

	for _, account := range d.accounts {
		if account.deactivated || account.organizationID != organizationID {
			continue
		}

		refs = append(refs, model.NamedRef{ID: account.id, Name: account.name})
	}

	return refs, nil
}
