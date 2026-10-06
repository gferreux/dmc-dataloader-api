package sftpgo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/auth"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/memory"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/adapter/sftpgo"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/app"
	loadhandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/loadconfig"
	sftphandler "github.com/dekuple-labs/dmc-dataloader-api/internal/app/handler/sftpaccount"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/usecase/loadconfig"
)

const (
	testAPIKey       = "sftpgo-test-key"
	publisherBucket  = "dkp-dmc-publishers-raw-euw1-dev"
	advertiserBucket = "dkp-dmc-advertisers-raw-euw1-dev"
	passwordAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_.!@#%+="
	passwordKeptWarn = "existing password is kept"
	keysIgnoredWarn  = "public keys are only applied when the user is created"
	samplePublicKey  = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKeyForSFTPGo"
)

func TestCreateNewPublisherUser(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})

	missing := perform(http.MethodGet, "/api/v1/sftp-accounts/no-such-user", nil, fx.api)
	require.Equal(t, http.StatusOK, missing.Code, missing.Body.String())

	var absent model.SFTPAccountView
	decode(t, missing, &absent)
	assert.False(t, absent.Exists)
	assert.Empty(t, absent.Bucket)
	assert.Empty(t, absent.VirtualFolders)
	assert.Empty(t, absent.Bases)
	assert.NotContains(t, missing.Body.String(), `"bucket"`)

	preview := perform(http.MethodPost, "/api/v1/sftp-accounts/preview", publisherRequest("generate", nil), fx.api)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())

	var plan model.SFTPPlan
	decode(t, preview, &plan)
	assert.Equal(t, model.SFTPActionCreate, plan.UserAction)
	assert.Equal(t, publisherBucket, plan.Bucket)
	assert.Equal(t, []string{"optin", "optout", "stop"}, plan.Subfolders)
	assert.Empty(t, plan.Warnings)
	require.Len(t, plan.Folders, 3)
	assert.Equal(t, "create", plan.Folders[0].Action)
	assert.Equal(t, "acme/base_fr/optin", plan.Folders[0].Name)
	assert.Equal(t, "/base_fr/optin", plan.Folders[0].VirtualPath)
	assert.Zero(t, fx.fake.writeCount())

	created := perform(http.MethodPost, "/api/v1/sftp-accounts", publisherRequest("generate", nil), fx.api)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var result model.SFTPApplyResult
	decode(t, created, &result)
	assert.Equal(t, model.SFTPActionCreated, result.UserAction)
	assert.True(t, result.Verified)
	assert.Empty(t, result.Missing)
	assert.Equal(t, 3, result.AddedVirtualFolders)
	assert.Len(t, result.FoldersCreated, 3)
	assert.Empty(t, result.FoldersExisting)
	assertPassword(t, result.GeneratedPassword)
	assert.Contains(t, created.Body.String(), result.GeneratedPassword)
	assert.NotContains(t, fx.logs.String(), result.GeneratedPassword)
	assert.NotContains(t, fx.logs.String(), testAPIKey)

	user := fx.fake.user("acme")
	assert.Equal(t, float64(1), user["status"])
	assert.Equal(t, "/srv/sftpgo/data/acme", user["home_dir"])
	assert.Equal(t, result.GeneratedPassword, user["password"])
	assertGCS(t, user["filesystem"], publisherBucket, "acme/")
	assertPermissionKeys(t, user["permissions"], "/", "/base_fr/optin", "/base_fr/optout", "/base_fr/stop")

	folder := fx.fake.folder("acme/base_fr/optin")
	assert.Equal(t, "acme - base_fr - optin", folder["description"])
	assertGCS(t, folder["filesystem"], publisherBucket, "acme/base_fr/optin/")

	got := perform(http.MethodGet, "/api/v1/sftp-accounts/acme", nil, fx.api)
	require.Equal(t, http.StatusOK, got.Code, got.Body.String())

	var view model.SFTPAccountView
	decode(t, got, &view)
	assert.True(t, view.Exists)
	assert.Equal(t, publisherBucket, view.Bucket)
	assert.Equal(t, []string{"base_fr"}, view.Bases)
	require.Len(t, view.VirtualFolders, 3)
	assert.Equal(t, "/base_fr/stop", view.VirtualFolders[2].VirtualPath)
}

func TestCreateWithPublicKeyAndNoPassword(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	body := publisherRequest(model.SFTPPasswordNone, []string{"  " + samplePublicKey + "  "})
	created := perform(http.MethodPost, "/api/v1/sftp-accounts", body, fx.api)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var result model.SFTPApplyResult
	decode(t, created, &result)
	assert.Empty(t, result.GeneratedPassword)
	assert.NotContains(t, created.Body.String(), "generatedPassword")
	assert.NotContains(t, fx.logs.String(), samplePublicKey)

	user := fx.fake.user("acme")
	_, hasPassword := user["password"]
	assert.False(t, hasPassword)
	assert.Equal(t, []any{samplePublicKey}, user["public_keys"])
}

func TestExistingUserGainsABase(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	fx.fake.seedAccount("acme", advertiserBucket, []string{"acme_fr"}, advertiserSubs())

	body := map[string]any{
		"user":         "acme",
		"base":         "acme_uk",
		"clientType":   "advertiser",
		"passwordMode": "generate",
		"publicKeys":   []string{samplePublicKey},
	}
	preview := perform(http.MethodPost, "/api/v1/sftp-accounts/preview", body, fx.api)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())

	var plan model.SFTPPlan
	decode(t, preview, &plan)
	assert.Equal(t, model.SFTPActionUpdate, plan.UserAction)
	assert.Equal(t, advertiserBucket, plan.Bucket)
	assert.Contains(t, plan.Warnings, passwordKeptWarn)
	assert.Contains(t, plan.Warnings, keysIgnoredWarn)
	assert.Zero(t, fx.fake.writeCount())

	updated := perform(http.MethodPost, "/api/v1/sftp-accounts", body, fx.api)
	require.Equal(t, http.StatusOK, updated.Code, updated.Body.String())

	var result model.SFTPApplyResult
	decode(t, updated, &result)
	assert.Equal(t, model.SFTPActionUpdated, result.UserAction)
	assert.True(t, result.Verified)
	assert.Equal(t, 4, result.AddedVirtualFolders)
	assert.Len(t, result.FoldersCreated, 4)
	assert.Empty(t, result.FoldersExisting)
	assert.Empty(t, result.GeneratedPassword)
	assert.NotContains(t, updated.Body.String(), "generatedPassword")

	stored := fx.fake.user("acme")
	_, hasPassword := stored["password"]
	assert.False(t, hasPassword)
	assert.Equal(t, "keep me", stored["description"])
	assert.Equal(t, "/srv/sftpgo/data/acme", stored["home_dir"])
	assert.NotContains(t, fx.fake.writeBodies(), samplePublicKey)
	assert.NotContains(t, fx.fake.writeBodies(), "hashed-secret")

	filters, ok := stored["filters"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"10.0.0.8"}, filters["allowed_ip"])
	assert.Len(t, stored["virtual_folders"].([]any), 8)
	assertPermissionKeys(t, stored["permissions"], "/", "/acme_fr/blacklists", "/acme_uk/sales")
	assertGCS(t, fx.fake.folder("acme/acme_uk/sales")["filesystem"], advertiserBucket, "acme/acme_uk/sales/")

	view := perform(http.MethodGet, "/api/v1/sftp-accounts/acme", nil, fx.api)
	require.Equal(t, http.StatusOK, view.Code, view.Body.String())

	var account model.SFTPAccountView
	decode(t, view, &account)
	assert.ElementsMatch(t, []string{"acme_fr", "acme_uk"}, account.Bases)
}

func TestRerunSameBaseIsUnchanged(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	fx.fake.seedAccount("acme", advertiserBucket, []string{"acme_fr"}, advertiserSubs())

	body := map[string]any{"user": "acme", "base": "acme_fr", "clientType": "advertiser"}
	writesBefore := fx.fake.writeCount()
	again := perform(http.MethodPost, "/api/v1/sftp-accounts", body, fx.api)
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())

	var result model.SFTPApplyResult
	decode(t, again, &result)
	assert.Equal(t, model.SFTPActionUnchanged, result.UserAction)
	assert.True(t, result.Verified)
	assert.Zero(t, result.AddedVirtualFolders)
	assert.Empty(t, result.FoldersCreated)
	assert.Len(t, result.FoldersExisting, 4)
	assert.Empty(t, result.GeneratedPassword)
	assert.Equal(t, writesBefore, fx.fake.writeCount())

	preview := perform(http.MethodPost, "/api/v1/sftp-accounts/preview", body, fx.api)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())

	var plan model.SFTPPlan
	decode(t, preview, &plan)
	assert.Equal(t, model.SFTPActionUnchanged, plan.UserAction)
	assert.Equal(t, "exists", plan.Folders[0].Action)
	assert.Contains(t, plan.Warnings, passwordKeptWarn)
}

func TestFolderBucketMismatch(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	fx.fake.seedFolder("acme/acme_fr/blacklists", "other-bucket")

	body := map[string]any{"user": "acme", "base": "acme_fr", "clientType": "advertiser"}
	for _, path := range []string{"/api/v1/sftp-accounts/preview", "/api/v1/sftp-accounts"} {
		response := perform(http.MethodPost, path, body, fx.api)
		require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
		assert.Equal(t, "bucket_mismatch", errorCode(t, response))
		assert.Contains(t, errorMessage(t, response), "folder acme/acme_fr/blacklists exists on bucket other-bucket")
		assert.Contains(t, errorMessage(t, response), advertiserBucket)
	}

	assert.Zero(t, fx.fake.writeCount())
	assert.Nil(t, fx.fake.user("acme"))
}

func TestUserBucketMismatch(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	fx.fake.seedUserOnly("acme", "other-bucket")

	body := map[string]any{"user": "acme", "base": "acme_fr", "clientType": "advertiser"}
	response := perform(http.MethodPost, "/api/v1/sftp-accounts", body, fx.api)
	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	assert.Equal(t, "bucket_mismatch", errorCode(t, response))
	assert.Contains(t, errorMessage(t, response), "user acme is on bucket other-bucket")
	assert.Zero(t, fx.fake.writeCount())
	assert.Nil(t, fx.fake.folder("acme/acme_fr/blacklists"))

	preview := perform(http.MethodPost, "/api/v1/sftp-accounts/preview", body, fx.api)
	require.Equal(t, http.StatusConflict, preview.Code, preview.Body.String())
	assert.Equal(t, "bucket_mismatch", errorCode(t, preview))
}

func TestPasswordNoneWithoutKeys(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	body := map[string]any{
		"user":         "acme",
		"base":         "base_fr",
		"clientType":   "publisher",
		"passwordMode": "none",
	}
	response := perform(http.MethodPost, "/api/v1/sftp-accounts", body, fx.api)
	require.Equal(t, http.StatusUnprocessableEntity, response.Code, response.Body.String())
	assert.Equal(t, "validation_error", errorCode(t, response))
	assert.Contains(t, response.Body.String(), "passwordMode")
	assert.Contains(t, response.Body.String(), "public key")
	assert.Zero(t, fx.fake.callCount())

	badName := perform(http.MethodPost, "/api/v1/sftp-accounts/preview", map[string]any{
		"user":       "Acme",
		"base":       "",
		"clientType": "referential",
	}, fx.api)
	require.Equal(t, http.StatusUnprocessableEntity, badName.Code, badName.Body.String())
	assert.Contains(t, badName.Body.String(), `"field":"user"`)
	assert.Contains(t, badName.Body.String(), `"field":"base"`)
	assert.Contains(t, badName.Body.String(), `"field":"clientType"`)
	assert.Zero(t, fx.fake.callCount())
}

func TestUnconfiguredReturns503AndOtherRoutesWork(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, false, auth.Noop{})
	for _, tc := range []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodGet, "/api/v1/sftp-accounts/acme", nil},
		{http.MethodPost, "/api/v1/sftp-accounts/preview", publisherRequest("generate", nil)},
		{http.MethodPost, "/api/v1/sftp-accounts", publisherRequest("generate", nil)},
	} {
		response := perform(tc.method, tc.path, tc.body, fx.api)
		require.Equal(t, http.StatusServiceUnavailable, response.Code, response.Body.String())
		assert.Equal(t, "sftpgo_unconfigured", errorCode(t, response))
		assert.Contains(t, errorMessage(t, response), "SFTPGO_URL")
		assert.Contains(t, errorMessage(t, response), "SFTPGO_API_KEY")
	}

	config := perform(http.MethodGet, "/api/v1/sftp-accounts/config", nil, fx.api)
	require.Equal(t, http.StatusOK, config.Code, config.Body.String())

	var view model.SFTPConfigView
	decode(t, config, &view)
	assert.False(t, view.Configured)
	assert.Equal(t, []string{"optin", "optout", "stop"}, view.ClientTypes.Publisher)
	assert.Equal(t, []string{"blacklists", "customers", "stores", "sales"}, view.ClientTypes.Advertiser)
	assert.Equal(t, publisherBucket, view.Buckets.Publisher)
	assert.Equal(t, advertiserBucket, view.Buckets.Advertiser)

	meta := perform(http.MethodGet, "/api/v1/meta", nil, fx.api)
	assert.Equal(t, http.StatusOK, meta.Code)
	health := perform(http.MethodGet, "/healthz", nil, fx.api)
	assert.Equal(t, http.StatusOK, health.Code)
	assert.Zero(t, fx.fake.callCount())
}

func TestUnconfiguredWhenEitherSettingMissing(t *testing.T) {
	t.Parallel()

	cases := []sftpgo.Config{
		{URL: "http://sftpgo", PublisherBucket: publisherBucket, AdvertiserBucket: advertiserBucket},
		{APIKey: testAPIKey, PublisherBucket: publisherBucket, AdvertiserBucket: advertiserBucket},
		{},
	}
	for _, cfg := range cases {
		svc := sftpgo.NewService(cfg, slog.Default())
		assert.False(t, svc.Config().Configured)

		_, err := svc.Apply(context.Background(), model.SFTPAccountRequest{
			User: "acme", Base: "base_fr", ClientType: "publisher",
		})
		assert.ErrorIs(t, err, model.ErrSFTPGoUnconfigured)
	}
}

func TestPasswordAndAPIKeyAreAbsentFromLogs(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, emailAuth{email: "ada@example.com"})
	created := perform(http.MethodPost, "/api/v1/sftp-accounts", publisherRequest("generate", nil), fx.api)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var result model.SFTPApplyResult
	decode(t, created, &result)
	require.NotEmpty(t, result.GeneratedPassword)

	logs := fx.logs.String()
	assert.NotContains(t, logs, result.GeneratedPassword)
	assert.NotContains(t, logs, testAPIKey)
	assert.NotContains(t, logs, "password")
	assert.Contains(t, logs, "ada@example.com")
	assert.Contains(t, logs, `"action":"created"`)
	assert.Contains(t, logs, `"user":"acme"`)
	assert.NotContains(t, fx.fake.apiKeysSeen(), "")
	assert.Contains(t, fx.fake.apiKeysSeen(), testAPIKey)
}

func TestUpstreamErrorIs502(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	fx.fake.fail(http.StatusInternalServerError, "  storage unavailable\n"+strings.Repeat("x", 20))

	response := perform(http.MethodPost, "/api/v1/sftp-accounts", publisherRequest("generate", nil), fx.api)
	require.Equal(t, http.StatusBadGateway, response.Code, response.Body.String())
	assert.Equal(t, "sftpgo_error", errorCode(t, response))
	assert.Contains(t, errorMessage(t, response), "storage unavailable")
	assert.NotContains(t, errorMessage(t, response), testAPIKey)
	assert.Zero(t, fx.fake.writeCount())
}

func TestConfigRouteIsNotAUsernameLookup(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	response := perform(http.MethodGet, "/api/v1/sftp-accounts/config", nil, fx.api)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"configured":true`)
	assert.Contains(t, response.Body.String(), `"clientTypes"`)
	assert.NotContains(t, response.Body.String(), `"exists"`)
}

func TestSFTPConfigRouteUsesAPIPrefix(t *testing.T) {
	t.Parallel()

	fx := newFixture(t, true, auth.Noop{})
	response := perform(http.MethodGet, "/api/v1/sftp-accounts/config", nil, fx.api)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Contains(t, response.Body.String(), `"configured":true`)

	legacy := perform(http.MethodGet, "/sftp-accounts/config", nil, fx.api)
	assert.Equal(t, http.StatusNotFound, legacy.Code, legacy.Body.String())
	assert.Equal(t, "not_found", errorCode(t, legacy))
}

func TestProvideServiceUsesDeriveBuckets(t *testing.T) {
	t.Parallel()

	cfg := model.AppConfig{Derive: model.DevDeriveConfig()}
	cfg.SFTPGo.URL = "http://sftpgo.test"
	cfg.SFTPGo.APIKey = testAPIKey
	cfg.SFTPGo.HomeRoot = "/data/sftp"

	svc := sftpgo.ProvideService(cfg, slog.Default())
	view := svc.Config()
	assert.True(t, view.Configured)
	assert.Equal(t, cfg.Derive.Publisher.RawBucket, view.Buckets.Publisher)
	assert.Equal(t, cfg.Derive.Advertiser.RawBucket, view.Buckets.Advertiser)
}

type emailAuth struct {
	email string
}

func (a emailAuth) Authenticate(context.Context, port.Credentials) (port.Principal, error) {
	return port.Principal{Email: a.email}, nil
}

type fixture struct {
	fake *fakeSFTPGo
	api  http.Handler
	logs *bytes.Buffer
}

func newFixture(t *testing.T, configured bool, authn port.Authenticator) *fixture {
	t.Helper()

	fake := newFake(testAPIKey)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(logs, nil))
	cfg := sftpgo.Config{
		HomeRoot:         "/srv/sftpgo/data",
		PublisherBucket:  publisherBucket,
		AdvertiserBucket: advertiserBucket,
	}
	if configured {
		cfg.URL = server.URL
		cfg.APIKey = testAPIKey
	}

	svc := sftpgo.NewService(cfg, logger)
	repo := memory.NewRepository()
	directory := memory.NewDirectory()
	usecase := loadconfig.NewUsecase(repo, directory, model.DevDeriveConfig(), slog.New(slog.DiscardHandler))
	handler := app.New(
		model.AppConfig{},
		loadhandler.NewHandler(usecase),
		sftphandler.NewHandler(svc),
		authn,
	).HTTPHandler()

	return &fixture{fake: fake, api: handler, logs: logs}
}

func publisherRequest(mode string, keys []string) map[string]any {
	body := map[string]any{
		"user":       "acme",
		"base":       "base_fr",
		"clientType": "publisher",
	}
	if mode != "" {
		body["passwordMode"] = mode
	}

	if keys != nil {
		body["publicKeys"] = keys
	}

	return body
}

func advertiserSubs() []string {
	return []string{"blacklists", "customers", "stores", "sales"}
}

func assertPassword(t *testing.T, password string) {
	t.Helper()
	assert.Len(t, password, 24)
	for _, char := range password {
		assert.Contains(t, passwordAlphabet, string(char))
	}
}

func assertGCS(t *testing.T, raw any, bucket, prefix string) {
	t.Helper()

	filesystem, ok := raw.(map[string]any)
	require.True(t, ok)

	assert.Equal(t, float64(2), filesystem["provider"])

	gcs, ok := filesystem["gcsconfig"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, bucket, gcs["bucket"])
	assert.Equal(t, prefix, gcs["key_prefix"])
	assert.Equal(t, float64(1), gcs["automatic_credentials"])
}

func assertPermissionKeys(t *testing.T, raw any, paths ...string) {
	t.Helper()

	perms, ok := raw.(map[string]any)
	require.True(t, ok)
	for _, path := range paths {
		_, exists := perms[path]
		assert.Truef(t, exists, "missing permission %s", path)
	}
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

	code, _ := errorParts(t, response)

	return code
}

func errorMessage(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()

	_, message := errorParts(t, response)

	return message
}

func errorParts(t *testing.T, response *httptest.ResponseRecorder) (string, string) {
	t.Helper()

	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decode(t, response, &body)

	return body.Error.Code, body.Error.Message
}

type capturedWrite struct {
	method string
	body   []byte
}

type fakeSFTPGo struct {
	mu       sync.Mutex
	apiKey   string
	folders  map[string]map[string]any
	users    map[string]map[string]any
	writes   int
	calls    int
	written  []capturedWrite
	keysSeen []string
	failCode int
	failBody string
}

func newFake(apiKey string) *fakeSFTPGo {
	return &fakeSFTPGo{
		apiKey:  apiKey,
		folders: map[string]map[string]any{},
		users:   map[string]map[string]any{},
	}
}

func (f *fakeSFTPGo) fail(status int, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failCode = status
	f.failBody = body
}

func (f *fakeSFTPGo) writeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.writes
}

func (f *fakeSFTPGo) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func (f *fakeSFTPGo) writeBodies() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var builder strings.Builder
	for _, write := range f.written {
		builder.Write(write.body)
	}

	return builder.String()
}

func (f *fakeSFTPGo) apiKeysSeen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string{}, f.keysSeen...)
}

func (f *fakeSFTPGo) user(name string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.users[name]
}

func (f *fakeSFTPGo) folder(name string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.folders[name]
}

func (f *fakeSFTPGo) seedFolder(name, bucket string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.folders[name] = map[string]any{
		"name": name,
		"filesystem": map[string]any{
			"provider": 2,
			"gcsconfig": map[string]any{
				"bucket":                bucket,
				"key_prefix":            name + "/",
				"automatic_credentials": 1,
			},
		},
	}
}

func (f *fakeSFTPGo) seedUserOnly(username, bucket string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.users[username] = map[string]any{
		"username":    username,
		"status":      1,
		"password":    "hashed-secret",
		"description": "keep me",
		"home_dir":    "/srv/sftpgo/data/" + username,
		"filesystem": map[string]any{
			"provider": 2,
			"gcsconfig": map[string]any{
				"bucket":     bucket,
				"key_prefix": username + "/",
			},
		},
		"permissions":     map[string]any{"/": []any{"list"}},
		"virtual_folders": []any{},
		"filters":         map[string]any{"allowed_ip": []any{"10.0.0.8"}},
	}
}

func (f *fakeSFTPGo) seedAccount(username, bucket string, bases, subs []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	perms := map[string]any{"/": []any{"list"}}

	var folders []any
	for _, base := range bases {
		for _, sub := range subs {
			name := username + "/" + base + "/" + sub
			virtualPath := "/" + base + "/" + sub
			f.folders[name] = map[string]any{
				"name": name,
				"filesystem": map[string]any{
					"provider": 2,
					"gcsconfig": map[string]any{
						"bucket":                bucket,
						"key_prefix":            name + "/",
						"automatic_credentials": 1,
					},
				},
			}
			folders = append(folders, map[string]any{
				"name":         name,
				"virtual_path": virtualPath,
				"quota_size":   -1,
				"quota_files":  -1,
				"filesystem":   map[string]any{"provider": 2},
			})
			perms[virtualPath] = []any{"list", "upload"}
		}
	}

	f.users[username] = map[string]any{
		"username":    username,
		"status":      1,
		"password":    "hashed-secret",
		"description": "keep me",
		"home_dir":    "/srv/sftpgo/data/" + username,
		"filesystem": map[string]any{
			"provider": 2,
			"gcsconfig": map[string]any{
				"bucket":     bucket,
				"key_prefix": username + "/",
			},
		},
		"permissions":     perms,
		"virtual_folders": folders,
		"filters":         map[string]any{"allowed_ip": []any{"10.0.0.8"}},
	}
}

func (f *fakeSFTPGo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	f.keysSeen = append(f.keysSeen, r.Header.Get("X-SFTPGO-API-KEY"))

	if r.Header.Get("X-SFTPGO-API-KEY") != f.apiKey {
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	}

	if f.failCode != 0 {
		http.Error(w, f.failBody, f.failCode)

		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v2")
	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/folders/"):
		f.readFolder(w, strings.TrimPrefix(path, "/folders/"))
	case r.Method == http.MethodPost && path == "/folders":
		f.createFolder(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/users/"):
		f.readUser(w, strings.TrimPrefix(path, "/users/"))
	case r.Method == http.MethodPost && path == "/users":
		f.createUser(w, r)
	case r.Method == http.MethodPut && strings.HasPrefix(path, "/users/"):
		f.updateUser(w, r, strings.TrimPrefix(path, "/users/"))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeSFTPGo) readFolder(w http.ResponseWriter, name string) {
	folder, ok := f.folders[name]
	if !ok {
		http.NotFound(w, nil)

		return
	}

	writeJSON(w, http.StatusOK, folder)
}

func (f *fakeSFTPGo) readUser(w http.ResponseWriter, username string) {
	user, ok := f.users[username]
	if !ok {
		http.NotFound(w, nil)

		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (f *fakeSFTPGo) createFolder(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	f.writes++
	f.written = append(f.written, capturedWrite{method: r.Method, body: body})

	var folder map[string]any
	if err := json.Unmarshal(body, &folder); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	name, _ := folder["name"].(string)
	f.folders[name] = folder
	writeJSON(w, http.StatusCreated, folder)
}

func (f *fakeSFTPGo) createUser(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	f.writes++
	f.written = append(f.written, capturedWrite{method: r.Method, body: body})

	var user map[string]any
	if err := json.Unmarshal(body, &user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	name, _ := user["username"].(string)
	f.users[name] = user
	writeJSON(w, http.StatusCreated, user)
}

func (f *fakeSFTPGo) updateUser(w http.ResponseWriter, r *http.Request, username string) {
	body := readBody(r)
	f.writes++
	f.written = append(f.written, capturedWrite{method: r.Method, body: body})

	var user map[string]any
	if err := json.Unmarshal(body, &user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	f.users[username] = user
	writeJSON(w, http.StatusOK, user)
}

func readBody(r *http.Request) []byte {
	body, _ := io.ReadAll(r.Body)

	return body
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
