package loadconfig_test

import (
	"bytes"
	"context"
	"log/slog"
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

	uc, repo, _ := newUsecase()
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
	assert.Empty(t, stored.Kind)
	assert.Equal(t, "org-acme", created.Organization.ID)
	require.NotNil(t, created.Organization.Account)
	assert.Equal(t, "acc-demo", *created.Organization.Account)
	assert.NotEqual(t, "demo-project", created.Destination.ProjectID)
}

func TestValidationWarnings(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()

	first := validSales("acme:demo:sales")
	first.Patterns.Ingest = `^bucket/acme/sales/.*\.csv$`
	_, err := repo.Create(ctx, first)
	require.NoError(t, err)

	second := validSales("other:demo:sales")
	second.Patterns.Ingest = `^bucket/acme/sales/.*\.csv$`
	second.Patterns.Preprocess = "bucket/file.tar.gz"
	second.Mode = model.ModeIncremental
	second.BQParams.FieldDelimiter = `\t`
	second.Mappings["order_id"] = model.Mapping{Src: `CONCAT("xxx","xxx")`, Type: model.MappingTypeSQL}

	report, err := uc.Validate(ctx, second)
	require.NoError(t, err)
	assert.Empty(t, report.Errors)
	assertWarning(t, report, "patterns")
	assertWarning(t, report, "patterns.preprocess")
	assertWarning(t, report, "mappings")
	assertWarning(t, report, "mappings.order_id.src")
	assertWarning(t, report, "bqParams.fieldDelimiter")
}

func TestValidationDoesNotWarnForAnchoredDistinctPatterns(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()
	_, err := repo.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	other := validSales("other:demo:sales")
	other.Patterns.Ingest = `^bucket/other/sales/.*\.csv$`
	report, err := uc.Validate(ctx, other)
	require.NoError(t, err)
	assert.Empty(t, report.Errors)
	assert.Empty(t, report.Warnings)
}

func TestListFiltersAndTestPatternOrder(t *testing.T) {
	t.Parallel()

	uc, repo, _ := newUsecase()
	ctx := context.Background()

	later := validSales("b:demo:sales")
	later.Patterns.Ingest = `^bucket/shared/.*\.csv$`
	_, err := repo.Create(ctx, later)
	require.NoError(t, err)

	earlier := validSales("a:demo:sales")
	earlier.Patterns.Ingest = `^bucket/shared/.*\.csv$`
	_, err = repo.Create(ctx, earlier)
	require.NoError(t, err)

	publisher := validSales("acme:demo:optin")
	publisher.PublisherName = "Acme Publisher"
	publisher.Organization.Type = model.OrganizationTypePublisher
	publisher.Destination.DatasetID = "dkp_dmc_publishers_raw_eu_dev"
	publisher.Destination.TableID = "profiles"
	publisher.Patterns.Ingest = `^bucket/acme/optin/.*\.csv$`
	_, err = repo.Create(ctx, publisher)
	require.NoError(t, err)

	active, err := uc.List(ctx, model.ListFilter{})
	require.NoError(t, err)
	require.Len(t, active, 3)

	salesOnly, err := uc.List(ctx, model.ListFilter{ImportType: model.ImportSales})
	require.NoError(t, err)
	require.Len(t, salesOnly, 2)
	assert.Equal(t, "a:demo:sales", salesOnly[0].ID)

	queried, err := uc.List(ctx, model.ListFilter{Query: "publisher"})
	require.NoError(t, err)
	require.Len(t, queried, 1)

	result, err := uc.TestPattern(ctx, `^bucket/shared/file\.csv$`, "bucket/shared/file.csv")
	require.NoError(t, err)
	assert.True(t, result.Matches)
	require.NotNil(t, result.MatchingConfigID)
	assert.Equal(t, "a:demo:sales", *result.MatchingConfigID)
}

func TestReferentialTypeIsReadableAndRejectedOnWrite(t *testing.T) {
	t.Parallel()

	uc, repo, directory := newUsecase()
	ctx := context.Background()
	stored := validSales("acme:referential:robinson")
	stored.Organization.Type = "referential"
	stored.Destination.DatasetID = "dmc_raw_referentials_eu_dev"
	stored.Destination.TableID = "fr_robinson"
	_, err := repo.Create(ctx, stored)
	require.NoError(t, err)

	got, err := uc.Get(ctx, stored.ID)
	require.NoError(t, err)
	assert.Equal(t, model.OrganizationType("referential"), got.Organization.Type)

	listed, err := uc.List(ctx, model.ListFilter{})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, model.OrganizationType("referential"), listed[0].Organization.Type)

	update := stored
	update.Identity = model.Identity{}
	_, err = uc.Update(ctx, stored.ID, update)
	var validation *model.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, issueFields(validation.Issues), "organization.type")

	_, err = uc.Create(ctx, validSales("other:demo:sales"))
	require.NoError(t, err)
	directory.AddAccount("acc-other", "other", "org-acme")
	ignored := validSales("other:referential:sales")
	ignored.NestedName = "other"
	ignored.Organization.Type = "referential"
	created, err := uc.Create(ctx, ignored)
	require.NoError(t, err)
	assert.Equal(t, model.OrganizationTypeAdvertiser, created.Organization.Type)
	assert.Equal(t, "org-acme", created.Organization.ID)
}

func TestUpdateAndDelete(t *testing.T) {
	t.Parallel()

	uc, _, _ := newUsecase()
	ctx := context.Background()
	created, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	updated := validSales("acme:demo:sales")
	updated.Mode = model.ModeOverwrite
	updated.Patterns.Preprocess = "ciblexo/.+"
	saved, err := uc.Update(ctx, "acme:demo:sales", updated)
	require.NoError(t, err)
	assert.Equal(t, model.ModeOverwrite, saved.Mode)
	assert.Equal(t, created.Patterns, saved.Patterns)
	assert.Equal(t, "acme:demo:sales", saved.PublisherName)

	missing := validSales("missing:demo:sales")
	_, err = uc.Update(ctx, "missing:demo:sales", missing)
	require.ErrorIs(t, err, model.ErrNotFound)

	require.NoError(t, uc.Delete(ctx, "acme:demo:sales"))
	require.ErrorIs(t, uc.Delete(ctx, "acme:demo:sales"), model.ErrNotFound)
}

func newUsecase() (port.LoadConfigUsecase, *memory.Repository, *memory.Directory) {
	repo := memory.NewRepository()
	directory := memory.NewDirectory()
	directory.AddOrganization("org-acme", "Acme", model.PartnerAdvertiser)
	directory.AddAccount("acc-demo", "demo", "org-acme")
	directory.AddOrganization("org-pub", "Ciblexo", model.PartnerPublisher)

	usecase := loadconfig.NewUsecase(repo, directory, model.DevDeriveConfig(), slog.Default())

	return usecase, repo, directory
}

func TestMutationsLogIAPEmail(t *testing.T) {
	t.Parallel()

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	repo := memory.NewRepository()
	directory := memory.NewDirectory()
	directory.AddOrganization("org-acme", "Acme", model.PartnerAdvertiser)
	directory.AddAccount("acc-demo", "demo", "org-acme")
	uc := loadconfig.NewUsecase(repo, directory, model.DevDeriveConfig(), logger)
	ctx := port.WithPrincipal(context.Background(), port.Principal{Email: "ada@example.com"})

	_, err := uc.Create(ctx, validSales("acme:demo:sales"))
	require.NoError(t, err)

	updated := validSales("acme:demo:sales")
	updated.Mode = model.ModeOverwrite
	_, err = uc.Update(ctx, "acme:demo:sales", updated)
	require.NoError(t, err)

	require.NoError(t, uc.Delete(ctx, "acme:demo:sales"))

	logged := logBuffer.String()
	assert.Contains(t, logged, `"email":"ada@example.com"`)
	assert.Contains(t, logged, "load config created")
	assert.Contains(t, logged, "load config updated")
	assert.Contains(t, logged, "load config deleted")
	assert.Contains(t, logged, "acme:demo:sales")
}

func validSales(id string) model.LoadConfig {
	return model.LoadConfig{
		ID:            id,
		PublisherName: "Acme",
		Identity: model.Identity{
			Kind:             model.PartnerAdvertiser,
			OrganizationName: "Acme",
			NestedName:       "demo",
			FileType:         model.ImportSales,
		},
		Mode:     model.ModeAppend,
		Patterns: model.Patterns{Ingest: `^bucket/acme/sales/.*\.csv$`},
		Destination: model.Destination{
			ProjectID: "demo-project",
			DatasetID: "dkp_dmc_advertisers_raw_eu_dev",
			TableID:   "sales",
		},
		Organization: model.Organization{Type: model.OrganizationTypeAdvertiser},
		BQParams: model.BQParams{
			FieldDelimiter:  ",",
			SkipLeadingRows: 1,
			SourceFormat:    model.SourceFormatCSV,
		},
		Mappings: map[string]model.Mapping{
			"order_id": {Src: "order_id", Type: model.MappingTypeRename},
		},
	}
}

func issueFields(issues []model.FieldIssue) []string {
	fields := make([]string, 0, len(issues))
	for _, issue := range issues {
		fields = append(fields, issue.Field)
	}

	return fields
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
