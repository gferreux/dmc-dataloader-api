package model

import (
	"regexp"
	"strings"
)

// Identity is the only input the console collects for a load_config's identity.
// Kind is advertiser or publisher. NestedName is an advertiser account or a
// publisher base. FileType is the import kind.
type Identity struct {
	Kind             string `json:"kind,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
	NestedName       string `json:"nestedName,omitempty"`
	FileType         string `json:"fileType,omitempty"`
}

// NamedRef is one autocomplete row: an organization, an account, or a publisher base.
type NamedRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// DerivedConfig is the server-computed block returned by POST /load-configs/derive.
type DerivedConfig struct {
	ID            string       `json:"id"`
	PublisherName string       `json:"publisherName"`
	Patterns      Patterns     `json:"patterns"`
	Notification  Notification `json:"notification"`
	Destination   Destination  `json:"destination"`
	Organization  Organization `json:"organization"`
	Warnings      []FieldIssue `json:"warnings"`
}

var conventionID = regexp.MustCompile(
	`^([a-z0-9_]+):([a-z0-9_]+):(optin|optout|blacklists|customers|stores|sales)$`,
)

var pathSegments = regexp.MustCompile(
	`(?:^|/)([a-z0-9_]+)/([a-z0-9_]+)/(optin|optout|blacklists|customers|stores|sales)(?:/|$)`,
)

// Normalized trims the wizard input and lowercases the closed sets.
func (id Identity) Normalized() Identity {
	id.Kind = strings.ToLower(strings.TrimSpace(id.Kind))
	id.FileType = strings.ToLower(strings.TrimSpace(id.FileType))
	id.OrganizationName = strings.TrimSpace(id.OrganizationName)
	id.NestedName = strings.TrimSpace(id.NestedName)

	return id
}

// Provided reports whether the client sent any wizard identity field.
func (id Identity) Provided() bool {
	id = id.Normalized()

	return id.Kind != "" || id.OrganizationName != "" || id.NestedName != "" || id.FileType != ""
}

// Issues reports wizard-input problems. An empty slice means the input can be derived.
func (id Identity) Issues() []FieldIssue {
	id = id.Normalized()

	issues := make([]FieldIssue, 0)
	if !ValidPartner(id.Kind) {
		issues = append(issues, FieldIssue{
			Field:   "kind",
			Message: "kind must be advertiser or publisher",
		})
	}

	if _, err := Slug(id.OrganizationName); err != nil {
		issues = append(issues, FieldIssue{
			Field:   "organizationName",
			Message: "organization name is empty after slug normalization",
		})
	}

	if _, err := Slug(id.NestedName); err != nil {
		issues = append(issues, FieldIssue{
			Field:   "nestedName",
			Message: "nested name is empty after slug normalization",
		})
	}

	switch {
	case id.FileType == "":
		issues = append(issues, FieldIssue{Field: "fileType", Message: "file type is required"})
	case ValidPartner(id.Kind) && !ValidImport(id.Kind, id.FileType):
		issues = append(issues, FieldIssue{
			Field:   "fileType",
			Message: "file type is not valid for the organization kind",
		})
	case !ValidPartner(id.Kind) && !KnownImport(id.FileType):
		issues = append(issues, FieldIssue{Field: "fileType", Message: "file type is not a known import kind"})
	}

	return issues
}

// SameIdentity reports whether both inputs slug to the same document key.
func SameIdentity(left, right Identity) bool {
	left = left.Normalized()

	right = right.Normalized()
	if left.Kind != right.Kind || left.FileType != right.FileType {
		return false
	}

	leftOrg, leftErr := Slug(left.OrganizationName)
	rightOrg, rightErr := Slug(right.OrganizationName)
	leftNested, leftNestedErr := Slug(left.NestedName)

	rightNested, rightNestedErr := Slug(right.NestedName)
	if leftErr != nil || rightErr != nil || leftNestedErr != nil || rightNestedErr != nil {
		return false
	}

	return leftOrg == rightOrg && leftNested == rightNested
}

// ParseIdentity reads the wizard input back from a stored document.
// The document id wins when it follows org:nested:fileType. Otherwise the
// path segments inside patterns.preprocess or patterns.ingest are used.
func ParseIdentity(cfg LoadConfig) (Identity, bool) {
	org, nested, fileType, ok := identityFromID(cfg.ID)
	if !ok {
		path, pathOK := PathIdentity(cfg.Patterns)
		if !pathOK {
			return Identity{}, false
		}

		org, nested, fileType = path.OrganizationName, path.NestedName, path.FileType
	}

	return identityFor(cfg, org, nested, fileType)
}

// PathIdentity reads org, nested name, and file type from pattern path segments.
// Kind comes from the file type. Publisher opt-in and advertiser sales cannot share one.
func PathIdentity(patterns Patterns) (Identity, bool) {
	org, nested, fileType, ok := identityFromPatterns(patterns)
	if !ok {
		return Identity{}, false
	}

	kind, known := PartnerForImport(fileType)
	if !known {
		return Identity{}, false
	}

	return Identity{
		Kind:             kind,
		OrganizationName: org,
		NestedName:       nested,
		FileType:         fileType,
	}, true
}

func identityFor(cfg LoadConfig, org, nested, fileType string) (Identity, bool) {
	kind := string(cfg.Organization.Type)
	if !ValidPartner(kind) {
		var known bool

		kind, known = PartnerForImport(fileType)
		if !known {
			return Identity{}, false
		}
	}

	if !ValidImport(kind, fileType) {
		return Identity{}, false
	}

	return Identity{
		Kind:             kind,
		OrganizationName: org,
		NestedName:       nested,
		FileType:         fileType,
	}, true
}

func identityFromID(id string) (string, string, string, bool) {
	matches := conventionID.FindStringSubmatch(strings.ToLower(strings.TrimSpace(id)))
	if matches == nil {
		return "", "", "", false
	}

	return matches[1], matches[2], matches[3], true
}

func identityFromPatterns(patterns Patterns) (string, string, string, bool) {
	if org, nested, fileType, ok := lastPathMatch(patterns.Preprocess); ok {
		return org, nested, fileType, true
	}

	return lastPathMatch(patterns.Ingest)
}

func lastPathMatch(pattern string) (string, string, string, bool) {
	matches := pathSegments.FindAllStringSubmatch(pattern, -1)
	if len(matches) == 0 {
		return "", "", "", false
	}

	last := matches[len(matches)-1]

	return last[1], last[2], last[3], true
}
