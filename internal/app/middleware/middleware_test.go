package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/app/middleware"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

func TestAuthenticateStoresIAPEmail(t *testing.T) {
	t.Parallel()

	var email string
	handler := middleware.Authenticate(emailAuth{email: "ada@example.com"})(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			email = port.PrincipalFrom(r.Context()).Email
			w.WriteHeader(http.StatusNoContent)
		}),
	)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/meta", nil)
	request.Header.Set("X-Goog-IAP-JWT-Assertion", "token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	require.Equal(t, http.StatusNoContent, response.Code)
	assert.Equal(t, "ada@example.com", email)
}

type emailAuth struct {
	email string
}

func (a emailAuth) Authenticate(context.Context, port.Credentials) (port.Principal, error) {
	return port.Principal{Email: a.email, Subject: "sub"}, nil
}
