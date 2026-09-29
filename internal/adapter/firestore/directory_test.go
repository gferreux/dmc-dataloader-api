package firestore

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestOrganizationAndAccountRefs(t *testing.T) {
	t.Parallel()

	_, ok := organizationRef("1", map[string]any{
		"name":        "Gadol",
		"type":        "advertiser",
		"deactivated": true,
	}, model.PartnerAdvertiser)
	assert.False(t, ok)

	ref, ok := organizationRef("01ORG", map[string]any{
		"name": "Gadól",
		"type": "Advertiser",
	}, model.PartnerAdvertiser)
	require.True(t, ok)
	assert.Equal(t, "01ORG", ref.ID)
	assert.Equal(t, "Gadól", ref.Name)

	_, ok = organizationRef("2", map[string]any{
		"name": "Ciblexo",
		"type": "publisher",
	}, model.PartnerAdvertiser)
	assert.False(t, ok)

	account, ok := accountRef("01ACC", map[string]any{
		"name":           "Lissac",
		"organizationId": "01ORG",
	}, "01ORG")
	require.True(t, ok)
	assert.Equal(t, "Lissac", account.Name)

	_, ok = accountRef("01ACC", map[string]any{
		"name":           "Lissac",
		"organizationId": "01ORG",
		"deactivated":    true,
	}, "01ORG")
	assert.False(t, ok)
}

func TestDirectoryListUsesRecords(t *testing.T) {
	t.Parallel()

	directory := &Directory{
		organizations: "organizations",
		accounts:      "accounts",
		list: func(_ context.Context, collection string) ([]documentRecord, error) {
			if collection == "organizations" {
				return []documentRecord{{
					id:   "01ORG",
					data: map[string]any{"name": "Gadol", "type": "advertiser"},
				}}, nil
			}

			return nil, errors.New("permission denied")
		},
	}

	refs, err := directory.ListOrganizations(context.Background(), model.PartnerAdvertiser)
	require.NoError(t, err)
	require.Len(t, refs, 1)
	assert.Equal(t, "Gadol", refs[0].Name)

	_, err = directory.ListAccounts(context.Background(), "01ORG")
	require.ErrorIs(t, err, model.ErrDirectoryUnavailable)

	canceled := &Directory{
		accounts: "accounts",
		list: func(context.Context, string) ([]documentRecord, error) {
			return nil, context.Canceled
		},
	}
	_, err = canceled.ListAccounts(context.Background(), "01ORG")
	require.ErrorIs(t, err, context.Canceled)
}
