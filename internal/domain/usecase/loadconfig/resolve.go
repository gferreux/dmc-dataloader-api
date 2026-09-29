package loadconfig

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func (u *usecase) resolve(
	ctx context.Context,
	identity model.Identity,
) (model.Organization, *model.FieldIssue, error) {
	identity = identity.Normalized()

	orgSlug, err := model.Slug(identity.OrganizationName)
	if err != nil {
		return model.Organization{}, nil, err
	}

	nestedSlug, err := model.Slug(identity.NestedName)
	if err != nil {
		return model.Organization{}, nil, err
	}

	refs, err := u.directory.ListOrganizations(ctx, identity.Kind)
	if err != nil {
		if !errors.Is(err, model.ErrDirectoryUnavailable) {
			return model.Organization{}, nil, err
		}

		u.warnDirectory(err, identity.Kind)

		return u.resolveFromConfigs(ctx, identity, orgSlug, nestedSlug, true)
	}

	match, ok := findBySlug(annotate(refs), orgSlug)
	if !ok {
		return u.resolveFromConfigs(ctx, identity, orgSlug, nestedSlug, false)
	}

	if identity.Kind == model.PartnerPublisher {
		return publisherOrganization(match.ID), nil, nil
	}

	accounts, err := u.directory.ListAccounts(ctx, match.ID)
	if err != nil {
		if !errors.Is(err, model.ErrDirectoryUnavailable) {
			return model.Organization{}, nil, err
		}

		u.warnDirectory(err, identity.Kind)

		return u.accountFromConfigs(ctx, identity, match.ID, orgSlug, nestedSlug)
	}

	account, ok := findBySlug(annotate(accounts), nestedSlug)
	if !ok {
		return model.Organization{}, nil, accountNotFound(identity)
	}

	return advertiserOrganization(match.ID, account.ID), nil, nil
}

func (u *usecase) accountFromConfigs(
	ctx context.Context,
	identity model.Identity,
	organizationID string,
	orgSlug string,
	nestedSlug string,
) (model.Organization, *model.FieldIssue, error) {
	match, err := u.fallbackMatch(ctx, identity.Kind, orgSlug, nestedSlug)
	if err != nil {
		return model.Organization{}, nil, err
	}

	if match.exact && match.orgID == organizationID && match.accountID != "" {
		return advertiserOrganization(organizationID, match.accountID), fallbackWarning(true), nil
	}

	return model.Organization{}, nil, accountNotFound(identity)
}

func (u *usecase) resolveFromConfigs(
	ctx context.Context,
	identity model.Identity,
	orgSlug string,
	nestedSlug string,
	unavailable bool,
) (model.Organization, *model.FieldIssue, error) {
	match, err := u.fallbackMatch(ctx, identity.Kind, orgSlug, nestedSlug)
	if err != nil {
		return model.Organization{}, nil, err
	}

	if !match.orgFound {
		return model.Organization{}, nil, organizationNotFound(identity)
	}

	if identity.Kind == model.PartnerAdvertiser && (!match.exact || match.accountID == "") {
		return model.Organization{}, nil, accountNotFound(identity)
	}

	warning := fallbackWarning(unavailable)
	u.logger.Warn(
		"resolved organization from load_config",
		"organizationId", match.orgID,
		"kind", identity.Kind,
		"warning", warning.Message,
	)

	if identity.Kind == model.PartnerPublisher {
		return publisherOrganization(match.orgID), warning, nil
	}

	return advertiserOrganization(match.orgID, match.accountID), warning, nil
}

type configMatch struct {
	orgID     string
	accountID string
	orgFound  bool
	exact     bool
}

func (u *usecase) fallbackMatch(ctx context.Context, kind, orgSlug, nestedSlug string) (configMatch, error) {
	configs, err := u.repo.List(ctx)
	if err != nil {
		return configMatch{}, err
	}

	sortByID(configs)

	var match configMatch

	for _, cfg := range configs {
		ident, ok := fallbackIdentity(cfg)
		if !ok || ident.Kind != kind || ident.OrganizationName != orgSlug || cfg.Organization.ID == "" {
			continue
		}

		if !match.orgFound {
			match.orgFound = true
			match.orgID = cfg.Organization.ID
		}

		if ident.NestedName != nestedSlug {
			continue
		}

		if !match.exact {
			match.exact = true
			match.orgID = cfg.Organization.ID
		}

		if cfg.Organization.ID != match.orgID {
			continue
		}

		if account := accountValue(cfg); account != "" && match.accountID == "" {
			match.accountID = account
		}
	}

	return match, nil
}

func fallbackIdentity(cfg model.LoadConfig) (model.Identity, bool) {
	ident, ok := model.PathIdentity(cfg.Patterns)
	if !ok {
		return model.ParseIdentity(cfg)
	}

	if cfg.Organization.Type.Valid() {
		ident.Kind = string(cfg.Organization.Type)
	}

	if !model.ValidImport(ident.Kind, ident.FileType) {
		return model.Identity{}, false
	}

	return ident, true
}

func annotate(refs []model.NamedRef) []model.NamedRef {
	annotated := make([]model.NamedRef, 0, len(refs))
	for _, ref := range refs {
		slug, err := model.Slug(ref.Name)
		if err != nil {
			continue
		}

		ref.Slug = slug
		annotated = append(annotated, ref)
	}

	return annotated
}

func findBySlug(refs []model.NamedRef, slug string) (model.NamedRef, bool) {
	var found model.NamedRef

	ok := false

	for _, ref := range refs {
		if ref.Slug != slug {
			continue
		}

		if !ok || ref.ID < found.ID {
			found = ref
			ok = true
		}
	}

	return found, ok
}

func publisherOrganization(id string) model.Organization {
	empty := ""

	return model.Organization{
		ID:      id,
		Account: &empty,
		Type:    model.OrganizationTypePublisher,
	}
}

func advertiserOrganization(id, accountID string) model.Organization {
	account := accountID

	return model.Organization{
		ID:      id,
		Account: &account,
		Type:    model.OrganizationTypeAdvertiser,
	}
}

func accountValue(cfg model.LoadConfig) string {
	if cfg.Organization.Account == nil {
		return ""
	}

	return strings.TrimSpace(*cfg.Organization.Account)
}

func organizationNotFound(identity model.Identity) error {
	return &model.ValidationError{Issues: []model.FieldIssue{{
		Field: "organizationName",
		Message: fmt.Sprintf(
			"organization %q (%s) was not found",
			identity.OrganizationName,
			identity.Kind,
		),
	}}}
}

func accountNotFound(identity model.Identity) error {
	return &model.ValidationError{Issues: []model.FieldIssue{{
		Field: "nestedName",
		Message: fmt.Sprintf(
			"account %q was not found for organization %q",
			identity.NestedName,
			identity.OrganizationName,
		),
	}}}
}

func fallbackWarning(unavailable bool) *model.FieldIssue {
	message := "organization was not in the directory; reused organization id from an existing load config"
	if unavailable {
		message = "organization directory was unavailable; reused organization id from an existing load config"
	}

	return &model.FieldIssue{Field: "organization.id", Message: message}
}

func (u *usecase) warnDirectory(err error, kind string) {
	u.logger.Warn(
		"organization directory unavailable; using load_config fallback",
		"error", err,
		"kind", kind,
	)
}
