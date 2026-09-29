// Package auth provides pluggable request authenticators.
package auth

import (
	"context"
	"errors"

	"google.golang.org/api/idtoken"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/port"
)

// Noop accepts every caller. Use it when IAP or Cloud Run IAM already guards the service.
type Noop struct{}

// Authenticate implements port.Authenticator.
func (Noop) Authenticate(context.Context, port.Credentials) (port.Principal, error) {
	return port.Principal{}, nil
}

// TokenValidator checks an IAP JWT. Tests inject a fake.
type TokenValidator func(ctx context.Context, token, audience string) (port.Principal, error)

// IAP authenticates the X-Goog-IAP-JWT-Assertion header.
// When audience is empty the header must be present but is not verified.
type IAP struct {
	audience string
	validate TokenValidator
}

// NewIAP returns an authenticator that uses Google's ID token validator.
func NewIAP(audience string) *IAP {
	return &IAP{audience: audience, validate: validateIAPToken}
}

// NewIAPWithValidator returns an authenticator that uses a custom validator.
func NewIAPWithValidator(audience string, validate TokenValidator) *IAP {
	return &IAP{audience: audience, validate: validate}
}

// Authenticate implements port.Authenticator.
func (a *IAP) Authenticate(ctx context.Context, creds port.Credentials) (port.Principal, error) {
	if creds.IAPJWT == "" {
		return port.Principal{}, model.ErrUnauthenticated
	}

	if a.audience == "" {
		return port.Principal{Subject: "iap"}, nil
	}

	principal, err := a.validate(ctx, creds.IAPJWT, a.audience)
	if err != nil {
		return port.Principal{}, model.ErrUnauthenticated
	}

	return principal, nil
}

func validateIAPToken(ctx context.Context, token, audience string) (port.Principal, error) {
	payload, err := idtoken.Validate(ctx, token, audience)
	if err != nil {
		return port.Principal{}, err
	}

	email, _ := payload.Claims["email"].(string)

	return port.Principal{Subject: payload.Subject, Email: email}, nil
}

var errUnsupportedAuthMode = errors.New("unsupported auth mode")

// NewAuthenticator selects the implementation configured for the process.
func NewAuthenticator(cfg model.AppConfig) (port.Authenticator, error) {
	switch cfg.Auth.Mode {
	case "none":
		return Noop{}, nil
	case "iap":
		return NewIAP(cfg.Auth.IAPAudience), nil
	default:
		return nil, errUnsupportedAuthMode
	}
}
