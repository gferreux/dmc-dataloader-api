package loadconfig_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/auth"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/memory"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/app"
	loadhandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/loadconfig"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/loadconfig"
)

func TestHealthzDoesNotRequireAuth(t *testing.T) {
	t.Parallel()

	handler := newAPI(denyAuth{})
	response := perform(http.MethodGet, "/healthz", nil, handler)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "OK", response.Body.String())
}

func TestIAPAuthRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	handler := newAPI(auth.NewIAP("audience"))
	response := perform(http.MethodGet, "/api/v1/meta", nil, handler)
	assert.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, "unauthorized", errorCode(t, response))
}

func TestCRUDAndRouting(t *testing.T) {
	t.Parallel()

	handler := newAPI(auth.Noop{})
	missing := perform(http.MethodGet, "/api/v1/load-configs/missing:demo:sales", nil, handler)
	assert.Equal(t, http.StatusNotFound, missing.Code)

	created := perform(http.MethodPost, "/api/v1/load-configs", validSales("acme:demo:sales"), handler)
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, "/api/v1/load-configs/acme:demo:sales", created.Header().Get("Location"))

	again := perform(http.MethodPost, "/api/v1/load-configs", validSales("acme:demo:sales"), handler)
	assert.Equal(t, http.StatusConflict, again.Code)

	got := perform(http.MethodGet, "/api/v1/load-configs/acme%3Ademo%3Asales", nil, handler)
	require.Equal(t, http.StatusOK, got.Code)
	var body model.LoadConfig
	decode(t, got, &body)
	assert.Equal(t, "acme:demo:sales", body.ID)
	assert.Equal(t, model.ImportSales, body.ImportType)
	assert.Equal(t, model.SourceFormatCSV, body.BQParams.SourceFormat)
	assert.Contains(t, got.Body.String(), `"sourceFormat":0`)

	invalid := perform(http.MethodPost, "/api/v1/load-configs", model.LoadConfig{ID: "bad id"}, handler)
	assert.Equal(t, http.StatusUnprocessableEntity, invalid.Code)
	assert.Equal(t, "validation_error", errorCode(t, invalid))

	updated := validSales("acme:demo:sales")
	updated.PublisherName = "Renamed"
	put := perform(http.MethodPut, "/api/v1/load-configs/acme:demo:sales", updated, handler)
	require.Equal(t, http.StatusOK, put.Code)

	deleted := perform(http.MethodDelete, "/api/v1/load-configs/acme:demo:sales", nil, handler)
	assert.Equal(t, http.StatusNoContent, deleted.Code)
	assert.Empty(t, deleted.Body.String())
}

func TestValidateReturnsWarningsWithoutFailingTheWrite(t *testing.T) {
	t.Parallel()

	handler, repo, directory := newStack(auth.Noop{})
	directory.AddAccount("acc-lissac", "lissac", "org-acme")
	first := validSales("seed:one:sales")
	_, err := repo.Create(context.Background(), first)
	require.NoError(t, err)

	second := validSales("other:demo:sales")
	second.NestedName = "lissac"
	validated := perform(http.MethodPost, "/api/v1/load-configs/validate", second, handler)
	require.Equal(t, http.StatusOK, validated.Code)
	var report model.ValidationReport
	decode(t, validated, &report)
	assert.Empty(t, report.Errors)
	assert.NotEmpty(t, report.Warnings)

	created := perform(http.MethodPost, "/api/v1/load-configs", second, handler)
	assert.Equal(t, http.StatusCreated, created.Code)
}

func TestTestPatternAndMeta(t *testing.T) {
	t.Parallel()

	handler := newAPI(auth.Noop{})
	cfg := validSales("acme:demo:sales")
	require.Equal(t, http.StatusCreated, perform(http.MethodPost, "/api/v1/load-configs", cfg, handler).Code)

	const sample = "dkp-dmc-advertisers-staging-euw1-dev/data/2024-06-01T12:00:00Z/acme/demo/sales/orders.csv"
	response := perform(http.MethodPost, "/api/v1/load-configs/test-pattern", map[string]string{
		"pattern": `^dkp-dmc-advertisers-staging-euw1-dev/data/2024-06-01T12:00:00Z/acme/demo/sales/orders\.csv$`,
		"path":    sample,
	}, handler)
	require.Equal(t, http.StatusOK, response.Code)
	var result model.PatternTest
	decode(t, response, &result)
	assert.True(t, result.Matches)
	require.NotNil(t, result.MatchingConfigID)
	assert.Equal(t, "acme:demo:sales", *result.MatchingConfigID)

	meta := perform(http.MethodGet, "/api/v1/meta", nil, handler)
	require.Equal(t, http.StatusOK, meta.Code)
	assert.Contains(t, meta.Body.String(), `"RENAME"`)
	assert.Contains(t, meta.Body.String(), `"ARRAY"`)
	assert.Contains(t, meta.Body.String(), `"sourceFormats"`)
	assert.NotContains(t, meta.Body.String(), `"bqType"`)

	templates := perform(http.MethodGet, "/api/v1/templates", nil, handler)
	require.Equal(t, http.StatusOK, templates.Code)
	assert.Contains(t, templates.Body.String(), `"sha256_mobile_phone"`)
	assert.NotContains(t, templates.Body.String(), "needsConfirmation")
}

func TestDeriveOrganizationsAndLegacyPattern(t *testing.T) {
	t.Parallel()

	handler, repo, _ := newStack(auth.Noop{})

	orgs := perform(http.MethodGet, "/api/v1/organizations?type=advertiser", nil, handler)
	require.Equal(t, http.StatusOK, orgs.Code)
	var orgItems []model.NamedRef
	decode(t, orgs, &orgItems)
	require.Len(t, orgItems, 1)
	assert.Equal(t, "org-acme", orgItems[0].ID)
	assert.Equal(t, "Acme", orgItems[0].Name)
	assert.Equal(t, "acme", orgItems[0].Slug)

	missingType := perform(http.MethodGet, "/api/v1/organizations", nil, handler)
	assert.Equal(t, http.StatusBadRequest, missingType.Code)

	accounts := perform(http.MethodGet, "/api/v1/organizations/org-acme/accounts", nil, handler)
	require.Equal(t, http.StatusOK, accounts.Code)
	var accountItems []model.NamedRef
	decode(t, accounts, &accountItems)
	require.Len(t, accountItems, 1)
	assert.Equal(t, "acc-demo", accountItems[0].ID)
	assert.Equal(t, "demo", accountItems[0].Slug)

	derived := perform(http.MethodPost, "/api/v1/load-configs/derive", model.Identity{
		Kind:             model.PartnerPublisher,
		OrganizationName: "Ciblexo",
		NestedName:       "lissac",
		FileType:         model.ImportOptin,
	}, handler)
	require.Equal(t, http.StatusOK, derived.Code)
	var preview model.DerivedConfig
	decode(t, derived, &preview)
	assert.Equal(t, "ciblexo:lissac:optin", preview.ID)
	assert.Equal(t, "profiles", preview.Destination.TableID)
	require.NotNil(t, preview.Organization.Account)
	assert.Empty(t, *preview.Organization.Account)
	assert.Contains(t, derived.Body.String(), `"account":""`)

	missingAccount := perform(http.MethodPost, "/api/v1/load-configs/derive", model.Identity{
		Kind:             model.PartnerAdvertiser,
		OrganizationName: "Acme",
		NestedName:       "missing",
		FileType:         model.ImportSales,
	}, handler)
	assert.Equal(t, http.StatusUnprocessableEntity, missingAccount.Code)
	assert.Contains(t, missingAccount.Body.String(), "account")
	assert.Contains(t, missingAccount.Body.String(), "missing")

	created := perform(http.MethodPost, "/api/v1/load-configs", model.LoadConfig{
		Identity: model.Identity{
			Kind:             model.PartnerPublisher,
			OrganizationName: "Ciblexo",
			NestedName:       "lissac",
			FileType:         model.ImportOptin,
		},
		Mode:     model.ModeAppend,
		BQParams: model.BQParams{SourceFormat: model.SourceFormatCSV, FieldDelimiter: ","},
		Mappings: map[string]model.Mapping{
			"sha256_mobile_phone": {Src: "sha256_mobile_phone", Type: model.MappingTypeRename},
		},
		Destination: model.Destination{ProjectID: "ignored"},
	}, handler)
	require.Equal(t, http.StatusCreated, created.Code)
	var saved model.LoadConfig
	decode(t, created, &saved)
	assert.Equal(t, "ciblexo:lissac:optin", saved.ID)
	assert.Equal(t, model.PartnerPublisher, saved.Kind)
	assert.Equal(t, "lissac", saved.NestedName)
	assert.NotEqual(t, "ignored", saved.Destination.ProjectID)

	bases := perform(http.MethodGet, "/api/v1/organizations/ciblexo/bases?type=publisher", nil, handler)
	require.Equal(t, http.StatusOK, bases.Code)
	var baseItems []model.NamedRef
	decode(t, bases, &baseItems)
	require.Len(t, baseItems, 1)
	assert.Equal(t, "lissac", baseItems[0].Slug)
	assert.NotContains(t, bases.Body.String(), `"id"`)

	badBases := perform(http.MethodGet, "/api/v1/organizations/ciblexo/bases", nil, handler)
	assert.Equal(t, http.StatusBadRequest, badBases.Code)

	empty := ""
	_, err := repo.Create(context.Background(), model.LoadConfig{
		ID:            "ciblexo",
		PublisherName: "ciblexo",
		Mode:          model.ModeAppend,
		Patterns:      model.Patterns{Preprocess: "ciblexo/.+"},
		Destination: model.Destination{
			ProjectID: "demo",
			DatasetID: "dkp_dmc_publishers_raw_eu_dev",
			TableID:   "profiles",
		},
		Organization: model.Organization{
			ID:      "uuid-ciblexo",
			Account: &empty,
			Type:    model.OrganizationTypePublisher,
		},
		BQParams: model.BQParams{SourceFormat: model.SourceFormatCSV},
		Mappings: map[string]model.Mapping{
			"sha256_mobile_phone": {Src: "sha256_mobile_phone", Type: model.MappingTypeRename},
		},
	})
	require.NoError(t, err)

	got := perform(http.MethodGet, "/api/v1/load-configs/ciblexo", nil, handler)
	require.Equal(t, http.StatusOK, got.Code)
	assert.NotContains(t, got.Body.String(), `"organizationName"`)
	assert.Contains(t, got.Body.String(), "ciblexo/.+")

	updated := perform(http.MethodPut, "/api/v1/load-configs/ciblexo", map[string]any{
		"mode":     "OVERWRITE",
		"bqParams": map[string]any{"sourceFormat": 0, "skipLeadingRows": 1},
		"mappings": map[string]any{
			"sha256_mobile_phone": map[string]any{"src": "sha256_mobile_phone", "type": 0},
		},
		"patterns": map[string]any{"preprocess": "should-not-stick/.+"},
	}, handler)
	require.Equal(t, http.StatusOK, updated.Code)
	assert.Contains(t, updated.Body.String(), "ciblexo/.+")
	assert.Contains(t, updated.Body.String(), `"mode":"OVERWRITE"`)
	assert.NotContains(t, updated.Body.String(), "should-not-stick")
}

func TestCORSAndBadJSON(t *testing.T) {
	t.Parallel()

	handler := newAPI(auth.Noop{})
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/load-configs", nil)
	request.Header.Set("Origin", "http://localhost:4200")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "http://localhost:4200", response.Header().Get("Access-Control-Allow-Origin"))

	bad := perform(http.MethodPost, "/api/v1/load-configs", json.RawMessage(`{"unknown":true}`), handler)
	assert.Equal(t, http.StatusBadRequest, bad.Code)
}

func TestIAPValidatorFailure(t *testing.T) {
	t.Parallel()

	authenticator := auth.NewIAPWithValidator("aud", func(context.Context, string, string) (port.Principal, error) {
		return port.Principal{}, model.ErrUnauthenticated
	})
	_, err := authenticator.Authenticate(context.Background(), port.Credentials{IAPJWT: "token"})
	require.ErrorIs(t, err, model.ErrUnauthenticated)

	_, err = auth.NewIAP("").Authenticate(context.Background(), port.Credentials{IAPJWT: "token"})
	require.ErrorIs(t, err, model.ErrUnauthenticated)

	_, err = auth.NewAuthenticator(model.AppConfig{Auth: model.AuthConfig{Mode: "iap"}})
	require.Error(t, err)

	none, err := auth.NewAuthenticator(model.AppConfig{Auth: model.AuthConfig{Mode: "none"}})
	require.NoError(t, err)
	principal, err := none.Authenticate(context.Background(), port.Credentials{})
	require.NoError(t, err)
	assert.Empty(t, principal.Email)
}

type denyAuth struct{}

func (denyAuth) Authenticate(context.Context, port.Credentials) (port.Principal, error) {
	return port.Principal{}, model.ErrUnauthenticated
}

func newAPI(authn port.Authenticator) http.Handler {
	handler, _, _ := newStack(authn)

	return handler
}

func newStack(authn port.Authenticator) (http.Handler, *memory.Repository, *memory.Directory) {
	repo := memory.NewRepository()
	directory := memory.NewDirectory()
	directory.AddOrganization("org-acme", "Acme", model.PartnerAdvertiser)
	directory.AddAccount("acc-demo", "demo", "org-acme")
	directory.AddOrganization("org-ciblexo", "Ciblexo", model.PartnerPublisher)
	usecase := loadconfig.NewUsecase(repo, directory, model.DevDeriveConfig(), slog.Default())
	handler := loadhandler.NewHandler(usecase)

	httpHandler := app.New(model.AppConfig{
		Server: model.ServerConfig{CORSOrigins: "http://localhost:4200"},
	}, handler, authn).HTTPHandler()

	return httpHandler, repo, directory
}

func perform(method, path string, body any, handler http.Handler) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}

		reader = bytes.NewReader(payload)
	}

	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	return response
}

func decode(t *testing.T, response *httptest.ResponseRecorder, dst any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), dst))
}

func errorCode(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decode(t, response, &body)

	return body.Error.Code
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
		BQParams:     model.BQParams{SourceFormat: model.SourceFormatCSV, FieldDelimiter: ","},
		Mappings: map[string]model.Mapping{
			"order_id": {Src: "order_id", Type: model.MappingTypeRename},
		},
	}
}
