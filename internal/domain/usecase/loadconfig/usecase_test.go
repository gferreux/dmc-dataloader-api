package loadconfig_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/memory"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/loadconfig"
)

func TestCreateRejectsInvalidAndKeepsDerivedFieldsOutOfStorage(t *testing.T) {
	t.Parallel()

	uc, repo := newUsecase()
	ctx := context.Background()

	_, err := uc.Create(ctx, model.LoadConfig{PublisherName: "Acme"})
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.NotEmpty(t, validation.Issues)

	created, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)
	assert.Equal(t, model.PartnerAdvertiser, created.PartnerType)
	assert.Equal(t, model.ImportSales, created.ImportType)
	assert.False(t, created.CreateTime.IsZero())

	stored, err := repo.Get(ctx, created.ID)
	require.NoError(t, err)
	assert.Empty(t, stored.PartnerType)
	assert.Empty(t, stored.ImportType)
}

func TestValidationWarnings(t *testing.T) {
	t.Parallel()

	uc, _ := newUsecase()
	ctx := context.Background()

	first := validSales("acme:demo:sales")
	first.Patterns.Ingest = `^bucket/acme/sales/.*\.csv$`
	_, err := uc.Create(ctx, first)
	require.NoError(t, err)

	second := validSales("other:demo:sales")
	second.Patterns.Ingest = `^bucket/acme/sales/.*\.csv$`
	second.Patterns.Preprocess = "bucket/file.tar.gz"
	second.Mode = model.ModeOverwrite
	second.Incremental = model.BoolPtr(true)
	second.BQParams.FieldDelimiter = `\t`
	stringType := model.MappingTypeString
	second.Mappings["order_id"] = model.Mapping{Src: `CONCAT("xxx","xxx")`, Type: &stringType}

	report, err := uc.Validate(ctx, second)
	require.NoError(t, err)
	assert.Empty(t, report.Errors)
	assertWarning(t, report, "patterns")
	assertWarning(t, report, "patterns.preprocess")
	assertWarning(t, report, "incremental")
	assertWarning(t, report, "mappings")
	assertWarning(t, report, "mappings.order_id.src")
	assertWarning(t, report, "bqParams.fieldDelimiter")
}

func TestValidationDoesNotWarnForAnchoredDistinctPatterns(t *testing.T) {
	t.Parallel()

	uc, _ := newUsecase()
	ctx := context.Background()
	_, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	other := validSales("other:demo:sales")
	other.Patterns.Ingest = `^bucket/other/sales/.*\.csv$`
	report, err := uc.Validate(ctx, other)
	require.NoError(t, err)
	assert.Empty(t, report.Errors)
	assert.Empty(t, report.Warnings)
}

func TestDeactivatedConfigIsExcludedFromOverlap(t *testing.T) {
	t.Parallel()

	uc, _ := newUsecase()
	ctx := context.Background()
	inactive := validSales("acme:demo:sales")
	inactive.Deactivated = true
	_, err := uc.Create(ctx, inactive)
	require.NoError(t, err)

	report, err := uc.Validate(ctx, validSales("other:demo:sales"))
	require.NoError(t, err)
	assert.Empty(t, report.Warnings)
}

func TestListFiltersAndTestPatternOrder(t *testing.T) {
	t.Parallel()

	uc, _ := newUsecase()
	ctx := context.Background()

	later := validSales("b:demo:sales")
	later.Patterns.Ingest = `^bucket/shared/.*\.csv$`
	_, err := uc.Create(ctx, later)
	require.NoError(t, err)

	earlier := validSales("a:demo:sales")
	earlier.Deactivated = true
	earlier.Patterns.Ingest = `^bucket/shared/.*\.csv$`
	_, err = uc.Create(ctx, earlier)
	require.NoError(t, err)

	publisher := validSales("acme:demo:optin")
	publisher.PublisherName = "Acme Publisher"
	publisher.Destination.DatasetID = "dkp_dmc_publishers_raw_eu_dev"
	publisher.Destination.TableID = "profiles"
	publisher.Patterns.Ingest = `^bucket/acme/optin/.*\.csv$`
	_, err = uc.Create(ctx, publisher)
	require.NoError(t, err)

	active, err := uc.List(ctx, model.ListFilter{})
	require.NoError(t, err)
	require.Len(t, active, 2)

	salesOnly, err := uc.List(ctx, model.ListFilter{ImportType: model.ImportSales})
	require.NoError(t, err)
	require.Len(t, salesOnly, 1)
	assert.Equal(t, "b:demo:sales", salesOnly[0].ID)

	queried, err := uc.List(ctx, model.ListFilter{Query: "publisher"})
	require.NoError(t, err)
	require.Len(t, queried, 1)

	withInactive, err := uc.List(ctx, model.ListFilter{IncludeDeactivated: true, ImportType: model.ImportSales})
	require.NoError(t, err)
	assert.Len(t, withInactive, 2)

	result, err := uc.TestPattern(ctx, `^bucket/shared/file\.csv$`, "bucket/shared/file.csv")
	require.NoError(t, err)
	assert.True(t, result.Matches)
	require.NotNil(t, result.MatchingConfigID)
	assert.Equal(t, "b:demo:sales", *result.MatchingConfigID)
}

func TestUpdateAndDelete(t *testing.T) {
	t.Parallel()

	uc, _ := newUsecase()
	ctx := context.Background()
	_, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	updated := validSales("acme:demo:sales")
	updated.PublisherName = "Renamed"
	saved, err := uc.Update(ctx, "acme:demo:sales", updated)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", saved.PublisherName)

	missing := validSales("missing:demo:sales")
	_, err = uc.Update(ctx, "missing:demo:sales", missing)
	require.ErrorIs(t, err, model.ErrNotFound)

	require.NoError(t, uc.Delete(ctx, "acme:demo:sales"))
	require.ErrorIs(t, uc.Delete(ctx, "acme:demo:sales"), model.ErrNotFound)
}

func newUsecase() (port.LoadConfigUsecase, *memory.Repository) {
	repo := memory.NewRepository()

	return loadconfig.NewUsecase(repo), repo
}

func validSales(id string) model.LoadConfig {
	return model.LoadConfig{
		ID:            id,
		PublisherName: "Acme",
		Mode:          model.ModeAppend,
		Patterns:      model.Patterns{Ingest: `^bucket/acme/sales/.*\.csv$`},
		Destination: model.Destination{
			ProjectID: "demo-project",
			DatasetID: "dkp_dmc_advertisers_raw_eu_dev",
			TableID:   "sales",
		},
		BQParams: model.BQParams{
			FieldDelimiter:  ",",
			SkipLeadingRows: 1,
			SourceFormat:    model.SourceFormatCSV,
		},
		Mappings: map[string]model.Mapping{
			"order_id": {Src: "order_id", Type: model.MappingTypePtr(model.MappingTypeString)},
		},
	}
}

func assertWarning(t *testing.T, report model.ValidationReport, field string) {
	t.Helper()

	for _, issue := range report.Warnings {
		if issue.Field == field {
			return
		}
	}

	t.Fatalf("missing warning for %s in %#v", field, report.Warnings)
}
