package loadconfig_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestDeriveAdvertiserFromDirectory(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase()
	derived, err := uc.Derive(context.Background(), model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "Acme",
		NestedName:       "demo",
		FileType:         model.ImportCustomers,
	})
	require.NoError(t, err)
	assert.Equal(t, "acme:demo:customers", derived.ID)
	assert.Equal(t, derived.ID, derived.PublisherName)
	assert.Equal(t, "org-acme", derived.Organization.ID)
	require.NotNil(t, derived.Organization.Account)
	assert.Equal(t, "acc-demo", *derived.Organization.Account)
	assert.Equal(t, "customers", derived.Destination.TableID)
	assert.Empty(t, derived.Warnings)
}

func TestDerivePublisherAccountIsEmpty(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase()
	derived, err := uc.Derive(context.Background(), model.Identity{
		Kind:             model.PartnerPublisher,
		OrganizationName: "Ciblexo",
		NestedName:       "lissac",
		FileType:         model.ImportOptin,
	})
	require.NoError(t, err)
	assert.Equal(t, "ciblexo:lissac:optin", derived.ID)
	assert.Equal(t, "org-pub", derived.Organization.ID)
	require.NotNil(t, derived.Organization.Account)
	assert.Empty(t, *derived.Organization.Account)
	assert.Equal(t, "profiles", derived.Destination.TableID)
	assert.Empty(t, derived.Warnings)
}

func TestDeriveFallsBackToLoadConfig(t *testing.T) {
	t.Parallel()

	uc, repo, directory := newUsecase()
	directory.SetUnavailable()
	account := "acc-from-config"
	_, err := repo.Create(context.Background(), model.LoadConfig{
		ID:            "legacy-uuid",
		PublisherName: "legacy-uuid",
		Mode:          model.ModeAppend,
		Patterns: model.Patterns{
			Preprocess: `^dkp-dmc-publishers-raw-euw1-dev/ciblexo/lissac/optin/.+[.]csv$`,
		},
		Destination: model.Destination{
			ProjectID: "demo",
			DatasetID: "dkp_dmc_publishers_raw_eu_dev",
			TableID:   "profiles",
		},
		Organization: model.Organization{
			ID:      "uuid-ciblexo",
			Account: &account,
			Type:    model.OrganizationTypePublisher,
		},
		BQParams: model.BQParams{SourceFormat: model.SourceFormatCSV},
		Mappings: map[string]model.Mapping{
			"mobile_phone": {Src: "mobile_phone", Type: model.MappingTypeRename},
		},
	})
	require.NoError(t, err)

	derived, err := uc.Derive(context.Background(), model.Identity{
		Kind:             model.PartnerPublisher,
		OrganizationName: "Ciblexo",
		NestedName:       "other-base",
		FileType:         model.ImportOptout,
	})
	require.NoError(t, err)
	assert.Equal(t, "uuid-ciblexo", derived.Organization.ID)
	require.NotNil(t, derived.Organization.Account)
	assert.Empty(t, *derived.Organization.Account)
	assert.Equal(t, "optout", derived.Destination.TableID)
	require.NotEmpty(t, derived.Warnings)
	assert.Equal(t, "organization.id", derived.Warnings[0].Field)
}

func TestDeriveMissingAccountReturns422(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase()
	_, err := uc.Derive(context.Background(), model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "Acme",
		NestedName:       "missing",
		FileType:         model.ImportSales,
	})
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "nestedName", validation.Issues[0].Field)
	assert.Contains(t, validation.Issues[0].Message, "account")
	assert.Contains(t, validation.Issues[0].Message, "missing")
}

func TestDeriveMissingOrganizationReturns422(t *testing.T) {
	t.Parallel()

	uc, _, directory := newUsecase()
	directory.SetUnavailable()
	_, err := uc.Derive(context.Background(), model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "Unknown",
		NestedName:       "demo",
		FileType:         model.ImportSales,
	})
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "organizationName", validation.Issues[0].Field)
	assert.Contains(t, validation.Issues[0].Message, "Unknown")
}

func TestCreateIgnoresClientPlumbingAndConflicts(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()
	input := validSales("client:forced:sales")
	input.PublisherName = "client-name"
	input.Destination.ProjectID = "client-project"
	input.Patterns.Preprocess = "client/.+"
	input.Organization.ID = "client-org"

	created, err := uc.Create(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, "acme:demo:sales", created.ID)
	assert.Equal(t, "acme:demo:sales", created.PublisherName)
	assert.Equal(t, "org-acme", created.Organization.ID)
	assert.Equal(t, "dmc-raw-advertisers-dev-27c7", created.Destination.ProjectID)
	assert.Contains(t, created.Patterns.Preprocess, "dkp-dmc-advertisers-raw-euw1-dev/acme/demo/sales/")
	assert.Equal(t, model.PartnerAdvertiser, created.Kind)
	assert.Equal(t, "acme", created.OrganizationName)
	assert.Equal(t, "demo", created.NestedName)
	assert.Equal(t, model.ImportSales, created.FileType)

	_, err = uc.Create(ctx, input)
	require.ErrorIs(t, err, model.ErrConflict)

	stored, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.OrganizationName)
}

func TestUpdatePreservesLegacyPatternUntilIdentityChanges(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()
	empty := ""
	legacy := validSales("ciblexo")
	legacy.PublisherName = "ciblexo"
	legacy.Patterns = model.Patterns{Preprocess: "ciblexo/.+"}
	legacy.Organization = model.Organization{
		ID:      "uuid-ciblexo",
		Account: &empty,
		Type:    model.OrganizationTypePublisher,
	}
	legacy.Destination.DatasetID = "dkp_dmc_publishers_raw_eu_dev"
	legacy.Destination.TableID = "profiles"
	_, err := repo.Create(ctx, legacy)
	require.NoError(t, err)

	kept := legacy
	kept.Identity = model.Identity{}
	kept.Mode = model.ModeOverwrite
	saved, err := uc.Update(ctx, "ciblexo", kept)
	require.NoError(t, err)
	assert.Equal(t, "ciblexo/.+", saved.Patterns.Preprocess)
	assert.Equal(t, model.ModeOverwrite, saved.Mode)
	assert.Equal(t, "ciblexo", saved.ID)
	assert.Empty(t, saved.OrganizationName)

	moved, err := uc.Update(ctx, "ciblexo", model.LoadConfig{
		Identity: model.Identity{
			Kind:             model.PartnerPublisher,
			OrganizationName: "Ciblexo",
			NestedName:       "lissac",
			FileType:         model.ImportOptin,
		},
		Mode:     model.ModeAppend,
		BQParams: model.BQParams{SourceFormat: model.SourceFormatCSV},
		Mappings: legacy.Mappings,
	})
	require.NoError(t, err)
	assert.Equal(t, "ciblexo:lissac:optin", moved.ID)
	assert.NotEqual(t, "ciblexo/.+", moved.Patterns.Preprocess)
	assert.Contains(t, moved.Patterns.Preprocess, "ciblexo/lissac/optin/")
	_, err = repo.Get(ctx, "ciblexo")
	require.ErrorIs(t, err, model.ErrNotFound)
}

func TestUpdateKeepsStoredPatternsWhenIdentityIsUnchanged(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase()
	ctx := context.Background()
	created, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	updated := validSales("acme:demo:sales")
	updated.OrganizationName = "ACME"
	updated.Patterns.Ingest = "replaced/.+"
	updated.Destination.TableID = "other"
	saved, err := uc.Update(ctx, created.ID, updated)
	require.NoError(t, err)
	assert.Equal(t, created.Patterns, saved.Patterns)
	assert.Equal(t, created.Destination, saved.Destination)
	assert.Equal(t, created.Organization, saved.Organization)
}

func TestDeriveWarnsOnPatternOverlap(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()
	input := model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "Acme",
		NestedName:       "demo",
		FileType:         model.ImportSales,
	}
	built, err := model.BuildDerived(input, model.DevDeriveConfig())
	require.NoError(t, err)
	existing := validSales("other:kept:sales")
	existing.Patterns = built.Patterns
	_, err = repo.Create(ctx, existing)
	require.NoError(t, err)

	derived, err := uc.Derive(ctx, input)
	require.NoError(t, err)
	assert.NotEmpty(t, derived.Warnings)
	assert.Equal(t, "patterns", derived.Warnings[0].Field)
	assert.Contains(t, derived.Warnings[0].Message, "other:kept:sales")
}

func TestAutocompleteFallsBackWhenDirectoryIsDown(t *testing.T) {
	t.Parallel()

	uc, repo, directory := newUsecase()
	ctx := context.Background()
	account := "acc-demo"
	cfg := validSales("acme:demo:sales")
	cfg.Patterns = model.Patterns{
		Preprocess: "dkp-dmc-advertisers-raw-euw1-dev/acme/demo/sales/file.csv",
	}
	cfg.Organization.ID = "org-from-config"
	cfg.Organization.Account = &account
	_, err := repo.Create(ctx, cfg)
	require.NoError(t, err)

	publisher := validSales("pub")
	publisher.Patterns = model.Patterns{
		Preprocess: "dkp-dmc-publishers-raw-euw1-dev/ciblexo/lissac/optin/file.csv",
	}
	publisher.Organization = model.Organization{ID: "uuid-ciblexo", Type: model.OrganizationTypePublisher}
	empty := ""
	publisher.Organization.Account = &empty
	_, err = repo.Create(ctx, publisher)
	require.NoError(t, err)

	directory.SetUnavailable()

	orgs, err := uc.ListOrganizations(ctx, model.PartnerAdvertiser)
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	assert.Equal(t, "org-from-config", orgs[0].ID)
	assert.Equal(t, "acme", orgs[0].Slug)

	accounts, err := uc.ListAccounts(ctx, "org-from-config")
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	assert.Equal(t, "acc-demo", accounts[0].ID)
	assert.Equal(t, "demo", accounts[0].Slug)

	bases, err := uc.ListBases(ctx, "Ciblexo")
	require.NoError(t, err)
	require.Len(t, bases, 1)
	assert.Equal(t, "lissac", bases[0].Name)
	assert.Empty(t, bases[0].ID)
}

func TestListOrganizationsUsesTheDirectory(t *testing.T) {
	t.Parallel()

	uc, _, directory := newUsecase()
	directory.AddDeactivatedOrganization("org-dead", "Dead Org", model.PartnerAdvertiser)

	listed, err := uc.ListOrganizations(context.Background(), model.PartnerAdvertiser)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "org-acme", listed[0].ID)
	assert.Equal(t, "Acme", listed[0].Name)
	assert.Equal(t, "acme", listed[0].Slug)
}
