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

	handler := newAPI(auth.Noop{})
	first := validSales("acme:demo:sales")
	require.Equal(t, http.StatusCreated, perform(http.MethodPost, "/api/v1/load-configs", first, handler).Code)

	second := validSales("other:demo:sales")
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

	response := perform(http.MethodPost, "/api/v1/load-configs/test-pattern", map[string]string{
		"pattern": `^bucket/acme/sales/file\.csv$`,
		"path":    "bucket/acme/sales/file.csv",
	}, handler)
	require.Equal(t, http.StatusOK, response.Code)
	var result model.PatternTest
	decode(t, response, &result)
	assert.True(t, result.Matches)
	require.NotNil(t, result.MatchingConfigID)
	assert.Equal(t, "acme:demo:sales", *result.MatchingConfigID)

	meta := perform(http.MethodGet, "/api/v1/meta", nil, handler)
	require.Equal(t, http.StatusOK, meta.Code)
	assert.Contains(t, meta.Body.String(), `"mappingTypes"`)

	templates := perform(http.MethodGet, "/api/v1/templates", nil, handler)
	require.Equal(t, http.StatusOK, templates.Code)
	assert.Contains(t, templates.Body.String(), `"sha256_mobile_phone"`)
	assert.NotContains(t, templates.Body.String(), "needsConfirmation")
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
	repo := memory.NewRepository()
	usecase := loadconfig.NewUsecase(repo, slog.Default())
	handler := loadhandler.NewHandler(usecase)

	return app.New(model.AppConfig{
		Server: model.ServerConfig{CORSOrigins: "http://localhost:4200"},
	}, handler, authn).HTTPHandler()
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
		Mode:          model.ModeAppend,
		Patterns:      model.Patterns{Ingest: `^bucket/acme/sales/.*\.csv$`},
		Destination: model.Destination{
			ProjectID: "demo-project",
			DatasetID: "dkp_dmc_advertisers_raw_eu_dev",
			TableID:   "sales",
		},
		BQParams: model.BQParams{SourceFormat: model.SourceFormatCSV, FieldDelimiter: ","},
		Mappings: map[string]model.Mapping{
			"order_id": {Src: "order_id", Type: model.MappingTypePtr(model.MappingTypeString)},
		},
	}
}
