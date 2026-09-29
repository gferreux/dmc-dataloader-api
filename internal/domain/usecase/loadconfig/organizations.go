package loadconfig

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func (u *usecase) ListOrganizations(ctx context.Context, kind string) ([]model.NamedRef, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))

	refs, err := u.directory.ListOrganizations(ctx, kind)
	if err != nil {
		if !errors.Is(err, model.ErrDirectoryUnavailable) {
			return nil, err
		}

		u.warnDirectory(err, kind)

		return u.organizationsFromConfigs(ctx, kind)
	}

	return sortRefs(annotate(refs)), nil
}

func (u *usecase) ListAccounts(ctx context.Context, organizationID string) ([]model.NamedRef, error) {
	organizationID = strings.TrimSpace(organizationID)

	refs, err := u.directory.ListAccounts(ctx, organizationID)
	if err != nil {
		if !errors.Is(err, model.ErrDirectoryUnavailable) {
			return nil, err
		}

		u.logger.Warn(
			"organization directory unavailable; using load_config fallback",
			"error", err,
			"organizationId", organizationID,
		)

		return u.accountsFromConfigs(ctx, organizationID)
	}

	return sortRefs(annotate(refs)), nil
}

func (u *usecase) ListBases(ctx context.Context, orgSlug string) ([]model.NamedRef, error) {
	wanted, _ := model.Slug(orgSlug)
	if wanted == "" {
		return []model.NamedRef{}, nil
	}

	configs, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	seen := map[string]model.NamedRef{}

	for _, cfg := range configs {
		ident, ok := model.PathIdentity(cfg.Patterns)
		if !ok || ident.Kind != model.PartnerPublisher || ident.OrganizationName != wanted {
			continue
		}

		if cfg.Organization.Type.Valid() && cfg.Organization.Type != model.OrganizationTypePublisher {
			continue
		}

		if _, exists := seen[ident.NestedName]; exists {
			continue
		}

		seen[ident.NestedName] = model.NamedRef{Name: ident.NestedName, Slug: ident.NestedName}
	}

	return sortedRefs(seen), nil
}

func (u *usecase) organizationsFromConfigs(ctx context.Context, kind string) ([]model.NamedRef, error) {
	configs, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	sortByID(configs)

	seen := map[string]model.NamedRef{}

	for _, cfg := range configs {
		ident, ok := fallbackIdentity(cfg)
		if !ok || ident.Kind != kind || cfg.Organization.ID == "" {
			continue
		}

		if _, exists := seen[cfg.Organization.ID]; exists {
			continue
		}

		seen[cfg.Organization.ID] = model.NamedRef{
			ID:   cfg.Organization.ID,
			Name: ident.OrganizationName,
			Slug: ident.OrganizationName,
		}
	}

	return sortedRefs(seen), nil
}

func (u *usecase) accountsFromConfigs(ctx context.Context, organizationID string) ([]model.NamedRef, error) {
	configs, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	sortByID(configs)

	seen := map[string]model.NamedRef{}

	for _, cfg := range configs {
		if cfg.Organization.ID != organizationID || accountValue(cfg) == "" {
			continue
		}

		ident, ok := fallbackIdentity(cfg)
		if !ok || ident.Kind != model.PartnerAdvertiser {
			continue
		}

		accountID := accountValue(cfg)
		if _, exists := seen[accountID]; exists {
			continue
		}

		seen[accountID] = model.NamedRef{ID: accountID, Name: ident.NestedName, Slug: ident.NestedName}
	}

	return sortedRefs(seen), nil
}

func sortedRefs(seen map[string]model.NamedRef) []model.NamedRef {
	refs := make([]model.NamedRef, 0, len(seen))
	for _, ref := range seen {
		refs = append(refs, ref)
	}

	return sortRefs(refs)
}

func sortRefs(refs []model.NamedRef) []model.NamedRef {
	if refs == nil {
		refs = []model.NamedRef{}
	}

	slices.SortFunc(refs, func(left, right model.NamedRef) int {
		if cmp := strings.Compare(left.Slug, right.Slug); cmp != 0 {
			return cmp
		}

		if cmp := strings.Compare(left.Name, right.Name); cmp != 0 {
			return cmp
		}

		return strings.Compare(left.ID, right.ID)
	})

	return refs
}
