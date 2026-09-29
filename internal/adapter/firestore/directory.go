package firestore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

// Directory reads organizations and advertiser accounts from Firestore.
// Documents mirror dekuple-labs/dmc-domain: organizations have name, type, and the
// DocumentHeader deactivated flag; accounts have name and organizationId.
type Directory struct {
	organizations string
	accounts      string
	list          func(ctx context.Context, collection string) ([]documentRecord, error)
}

type documentRecord struct {
	id   string
	data map[string]any
}

// NewDirectory opens the organization source. FIRESTORE_EMULATOR_HOST applies,
// the same way it does for the load_config client.
func NewDirectory(ctx context.Context, cfg model.AppConfig) (*Directory, func(), error) {
	client, err := openClient(ctx, cfg.Organizations.ProjectID, cfg.Organizations.DatabaseID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating organization firestore client: %w", err)
	}

	directory := &Directory{
		organizations: cfg.Organizations.Collection,
		accounts:      cfg.Organizations.AccountsCollection,
		list: func(ctx context.Context, collection string) ([]documentRecord, error) {
			docs, err := client.Collection(collection).Documents(ctx).GetAll()
			if err != nil {
				return nil, err
			}

			records := make([]documentRecord, 0, len(docs))
			for _, doc := range docs {
				records = append(records, documentRecord{id: doc.Ref.ID, data: doc.Data()})
			}

			return records, nil
		},
	}
	cleanup := func() {
		_ = client.Close()
	}

	return directory, cleanup, nil
}

// ListOrganizations returns active organizations whose type equals kind.
func (d *Directory) ListOrganizations(ctx context.Context, kind string) ([]model.NamedRef, error) {
	records, err := d.list(ctx, d.organizations)
	if err != nil {
		return nil, directoryFailure("listing organizations", err)
	}

	refs := make([]model.NamedRef, 0)

	for _, record := range records {
		ref, ok := organizationRef(record.id, record.data, kind)
		if ok {
			refs = append(refs, ref)
		}
	}

	return refs, nil
}

// ListAccounts returns active accounts stored for one organization.
func (d *Directory) ListAccounts(ctx context.Context, organizationID string) ([]model.NamedRef, error) {
	records, err := d.list(ctx, d.accounts)
	if err != nil {
		return nil, directoryFailure("listing accounts", err)
	}

	refs := make([]model.NamedRef, 0)

	for _, record := range records {
		ref, ok := accountRef(record.id, record.data, organizationID)
		if ok {
			refs = append(refs, ref)
		}
	}

	return refs, nil
}

func organizationRef(id string, data map[string]any, kind string) (model.NamedRef, bool) {
	if boolField(data, "deactivated") {
		return model.NamedRef{}, false
	}

	kindValue := strings.ToLower(strings.TrimSpace(stringField(data, "type")))
	if kindValue != kind {
		return model.NamedRef{}, false
	}

	name := strings.TrimSpace(stringField(data, "name"))
	if name == "" {
		return model.NamedRef{}, false
	}

	return model.NamedRef{ID: id, Name: name}, true
}

func accountRef(id string, data map[string]any, organizationID string) (model.NamedRef, bool) {
	if boolField(data, "deactivated") {
		return model.NamedRef{}, false
	}

	if strings.TrimSpace(stringField(data, "organizationId")) != organizationID {
		return model.NamedRef{}, false
	}

	name := strings.TrimSpace(stringField(data, "name"))
	if name == "" {
		return model.NamedRef{}, false
	}

	return model.NamedRef{ID: id, Name: name}, true
}

func stringField(data map[string]any, key string) string {
	value, _ := data[key].(string)

	return value
}

func boolField(data map[string]any, key string) bool {
	value, _ := data[key].(bool)

	return value
}

func directoryFailure(action string, err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	return fmt.Errorf("%s: %w", action, errors.Join(model.ErrDirectoryUnavailable, err))
}
