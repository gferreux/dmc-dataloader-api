package model_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestSlug(t *testing.T) {
	t.Parallel()

	slug, err := model.Slug("  BigMat France ")
	require.NoError(t, err)
	assert.Equal(t, "bigmat_france", slug)

	slug, err = model.Slug("Gadól")
	require.NoError(t, err)
	assert.Equal(t, "gadol", slug)

	slug, err = model.Slug("L'Oréal")
	require.NoError(t, err)
	assert.Equal(t, "loreal", slug)

	slug, err = model.Slug("foo-bar baz")
	require.NoError(t, err)
	assert.Equal(t, "foo_bar_baz", slug)

	_, err = model.Slug(" !!! ")
	require.ErrorIs(t, err, model.ErrEmptySlug)
}

func TestBuildDerivedPatterns(t *testing.T) {
	t.Parallel()

	advertiser, err := model.BuildDerived(model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "BigMat France",
		NestedName:       "bigmat",
		FileType:         model.ImportSales,
	}, model.DevDeriveConfig())
	require.NoError(t, err)
	assert.Equal(t, "bigmat_france:bigmat:sales", advertiser.ID)
	assert.Equal(t, advertiser.ID, advertiser.PublisherName)
	assert.Equal(t, "dmc-raw-advertisers-dev-27c7", advertiser.Destination.ProjectID)
	assert.Equal(t, "dkp_dmc_advertisers_raw_eu_dev", advertiser.Destination.DatasetID)
	assert.Equal(t, "sales", advertiser.Destination.TableID)
	assert.Equal(t, "dmc-curated-inventory-dev-e6da", advertiser.Notification.ProjectID)
	assert.Equal(t, "dkp-dmc-data-loader-notifications-dev", advertiser.Notification.TopicID)
	assert.Nil(t, advertiser.Organization.Account)
	assert.Equal(t, model.OrganizationTypeAdvertiser, advertiser.Organization.Type)
	assert.Equal(t,
		"dkp-dmc-advertisers-raw-euw1-dev/bigmat_france/bigmat/sales/.+[.](csv|zip|gz|gzip|tgz|tar.gz|7z)",
		advertiser.Patterns.Preprocess,
	)
	assert.Equal(t,
		"dkp-dmc-advertisers-staging-euw1-dev/data/[0-9]{4}-[01][0-9]-[0-3][0-9]T[0-2][0-9]:[0-5][0-9]:[0-5][0-9]Z/bigmat_france/bigmat/sales/.+",
		advertiser.Patterns.Ingest,
	)
	assert.NotContains(t, advertiser.Patterns.Preprocess, "^")
	assert.NotContains(t, advertiser.Patterns.Ingest, "$")

	preprocess := regexp.MustCompile(advertiser.Patterns.Preprocess)
	for _, ext := range []string{"csv", "zip", "gz", "gzip", "tgz", "tar.gz", "7z"} {
		path := "dkp-dmc-advertisers-raw-euw1-dev/bigmat_france/bigmat/sales/orders." + ext
		assert.True(t, preprocess.MatchString(path), ext)
		assert.True(t, preprocess.MatchString("gs://"+path), "gs:// "+ext)
	}

	assert.False(t, preprocess.MatchString(
		"dkp-dmc-advertisers-raw-euw1-dev/bigmat_france/bigmat/sales/orders.txt",
	))
	assert.False(t, preprocess.MatchString(
		"dkp-dmc-advertisers-raw-euw1-dev/other/bigmat/sales/orders.csv",
	))

	ingest := regexp.MustCompile(advertiser.Patterns.Ingest)
	staged := "dkp-dmc-advertisers-staging-euw1-dev/data/2024-06-01T12:00:00Z/bigmat_france/bigmat/sales/orders.csv"
	assert.True(t, ingest.MatchString(staged))
	assert.True(t, ingest.MatchString("gs://"+staged))
	assert.False(t, ingest.MatchString(
		"dkp-dmc-advertisers-staging-euw1-dev/data/2024-06-01T12:00:00Z/bigmat_france/bigmat/customers/orders.csv",
	))

	publisher, err := model.BuildDerived(model.Identity{
		Kind:             model.PartnerPublisher,
		OrganizationName: "Ciblexo",
		NestedName:       "Lissac",
		FileType:         model.ImportOptin,
	}, model.DevDeriveConfig())
	require.NoError(t, err)
	assert.Equal(t, "ciblexo:lissac:optin", publisher.ID)
	assert.Equal(t, "profiles", publisher.Destination.TableID)
	assert.Equal(t, "dmc-raw-publishers-dev-c69c", publisher.Destination.ProjectID)
	assert.Equal(t, "dkp_dmc_publishers_raw_eu_dev", publisher.Destination.DatasetID)
	require.NotNil(t, publisher.Organization.Account)
	assert.Empty(t, *publisher.Organization.Account)
	assert.Equal(t,
		"dkp-dmc-publishers-raw-euw1-dev/ciblexo/lissac/optin/.+[.](csv|zip|gz|gzip|tgz|tar.gz|7z)",
		publisher.Patterns.Preprocess,
	)
	assert.Equal(t,
		"dkp-dmc-publishers-staging-euw1-dev/data/[0-9]{4}-[01][0-9]-[0-3][0-9]T[0-2][0-9]:[0-5][0-9]:[0-5][0-9]Z/ciblexo/lissac/optin/.+",
		publisher.Patterns.Ingest,
	)

	parsed, ok := model.ParseIdentity(publisher)
	require.True(t, ok)
	assert.Equal(t, model.PartnerPublisher, parsed.Kind)
	assert.Equal(t, "ciblexo", parsed.OrganizationName)
	assert.Equal(t, "lissac", parsed.NestedName)
	assert.Equal(t, model.ImportOptin, parsed.FileType)

	legacy := model.LoadConfig{ID: "ciblexo", Patterns: model.Patterns{Preprocess: "ciblexo/.+"}}
	_, ok = model.ParseIdentity(legacy)
	assert.False(t, ok)
}

func TestIdentityRejectsEmptySlugAndWrongFileType(t *testing.T) {
	t.Parallel()

	_, err := model.BuildDerived(model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "!!!",
		NestedName:       "bigmat",
		FileType:         model.ImportSales,
	}, model.DevDeriveConfig())
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "organizationName", validation.Issues[0].Field)

	_, err = model.BuildDerived(model.Identity{
		Kind:             model.PartnerPublisher,
		OrganizationName: "ciblexo",
		NestedName:       "lissac",
		FileType:         model.ImportSales,
	}, model.DevDeriveConfig())
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "fileType", validation.Issues[0].Field)
}
