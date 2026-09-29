package model

import (
	"maps"
	"slices"
	"strings"
	"time"
)

// Load modes stored on load_config.mode. The loader treats these as write dispositions.
const (
	ModeAppend      LoadMode = "APPEND"
	ModeIncremental LoadMode = "INCREMENTAL"
	ModeOverwrite   LoadMode = "OVERWRITE"
)

// Partner kinds and import kinds used for filtering and templates.
const (
	PartnerPublisher  = "publisher"
	PartnerAdvertiser = "advertiser"

	ImportOptin      = "optin"
	ImportOptout     = "optout"
	ImportBlacklists = "blacklists"
	ImportCustomers  = "customers"
	ImportStores     = "stores"
	ImportSales      = "sales"
)

// Organization types from dekuple-labs/dmc-domain/pkg/model/organization_type.go.
// The loader's Validate accepts only advertiser and publisher. One dev document
// stores "referential"; list and read return that value unchanged, and writes reject it.
const (
	OrganizationTypeAdvertiser OrganizationType = "advertiser"
	OrganizationTypePublisher  OrganizationType = "publisher"
)

// SourceFormat is bqParams.sourceFormat.
// The domain model stores an int: 0 is CSV, 1 is JSON.
type SourceFormat int

const (
	SourceFormatCSV  SourceFormat = 0
	SourceFormatJSON SourceFormat = 1
)

// DefaultProjectID is the dev GCP project used when a template needs a destination project.
const DefaultProjectID = "dmc-datastores-dev-becb"

// LoadMode is the Firestore mode string.
type LoadMode string

// OrganizationType is the partner kind stored on organization.type.
type OrganizationType string

// Valid reports whether the mode is one the loader accepts.
func (m LoadMode) Valid() bool {
	switch m {
	case ModeAppend, ModeIncremental, ModeOverwrite:
		return true
	default:
		return false
	}
}

// Valid reports whether the organization type is accepted by the loader's Validate.
func (t OrganizationType) Valid() bool {
	return t == OrganizationTypeAdvertiser || t == OrganizationTypePublisher
}

// Valid reports whether the source format is CSV (0) or JSON (1).
func (f SourceFormat) Valid() bool {
	return f == SourceFormatCSV || f == SourceFormatJSON
}

// Label returns CSV or JSON for a known source format.
func (f SourceFormat) Label() string {
	switch f {
	case SourceFormatCSV:
		return "CSV"
	case SourceFormatJSON:
		return "JSON"
	default:
		return ""
	}
}

// Modes returns the mode values exposed by GET /api/v1/meta.
func Modes() []LoadMode {
	return []LoadMode{ModeAppend, ModeIncremental, ModeOverwrite}
}

// SourceFormatInfo is one entry of GET /api/v1/meta sourceFormats.
type SourceFormatInfo struct {
	Value SourceFormat `json:"value"`
	Label string       `json:"label"`
}

// SourceFormats returns the source formats exposed by GET /api/v1/meta.
func SourceFormats() []SourceFormatInfo {
	return []SourceFormatInfo{
		{Value: SourceFormatCSV, Label: SourceFormatCSV.Label()},
		{Value: SourceFormatJSON, Label: SourceFormatJSON.Label()},
	}
}

// PartnerTypes returns the partner kinds exposed by GET /api/v1/meta.
func PartnerTypes() []string {
	return []string{PartnerPublisher, PartnerAdvertiser}
}

// ImportTypes returns the import kinds grouped by partner.
func ImportTypes() map[string][]string {
	return map[string][]string{
		PartnerPublisher:  {ImportOptin, ImportOptout},
		PartnerAdvertiser: {ImportBlacklists, ImportCustomers, ImportStores, ImportSales},
	}
}

// ValidPartner reports whether partner is a known partner kind.
func ValidPartner(partner string) bool {
	return partner == PartnerPublisher || partner == PartnerAdvertiser
}

// ValidImport reports whether importType belongs to partner.
func ValidImport(partner, importType string) bool {
	return slices.Contains(ImportTypes()[partner], importType)
}

// PartnerForImport returns the partner kind that owns a known import type.
func PartnerForImport(importType string) (string, bool) {
	for partner, imports := range ImportTypes() {
		if slices.Contains(imports, importType) {
			return partner, true
		}
	}

	return "", false
}

// KnownImport reports whether importType is one of the product import kinds.
func KnownImport(importType string) bool {
	_, ok := PartnerForImport(importType)

	return ok
}

// LoadConfig mirrors dekuple-labs/dmc-domain/pkg/model/data_loader_config.go.
//
// DocumentHeader is not a stored body. ID, CreateTime, and UpdateTime come from
// the Firestore snapshot. PartnerType and ImportType are derived for the console
// and are tagged firestore:"-" so they are not written.
//
// Dev documents also carry fields this struct does not declare, including
// deactivated, incremental, and mappings.<column>.isPartitionKey. Reads ignore
// them. Updates merge them back so a write does not delete them.
type LoadConfig struct {
	// Identity is the wizard input. It is not stored. Reads fill it when the
	// document id or patterns follow org:nested:fileType.
	Identity `firestore:"-"`

	ID          string    `firestore:"-" json:"id,omitempty"`
	CreateTime  time.Time `firestore:"-" json:"createTime,omitzero"`
	UpdateTime  time.Time `firestore:"-" json:"updateTime,omitzero"`
	PartnerType string    `firestore:"-" json:"partnerType,omitempty"`
	ImportType  string    `firestore:"-" json:"importType,omitempty"`

	PublisherName string             `firestore:"publisherName" json:"publisherName"`
	Patterns      Patterns           `firestore:"patterns"      json:"patterns"`
	Destination   Destination        `firestore:"destination"   json:"destination"`
	BQParams      BQParams           `firestore:"bqParams"      json:"bqParams"`
	Mappings      map[string]Mapping `firestore:"mappings"      json:"mappings"`
	Organization  Organization       `firestore:"organization"  json:"organization"`
	Notification  Notification       `firestore:"notification"  json:"notification"`
	Mode          LoadMode           `firestore:"mode"          json:"mode"`
}

// Patterns holds the regexes the loader matches against bucket/objectName.
type Patterns struct {
	Preprocess string `firestore:"preprocess" json:"preprocess,omitempty"`
	Ingest     string `firestore:"ingest"     json:"ingest,omitempty"`
}

// Destination is the BigQuery table that receives the file.
type Destination struct {
	ProjectID string `firestore:"projectId" json:"projectId"`
	DatasetID string `firestore:"datasetId" json:"datasetId"`
	TableID   string `firestore:"tableId"   json:"tableId"`
}

// Organization identifies the partner account on the document.
// Account is nullable, matching the domain model.
type Organization struct {
	ID      string           `firestore:"id"      json:"id"`
	Account *string          `firestore:"account" json:"account"`
	Type    OrganizationType `firestore:"type"    json:"type"`
}

// Notification is the Pub/Sub topic notified after a load.
type Notification struct {
	ProjectID string `firestore:"projectId" json:"projectId,omitempty"`
	TopicID   string `firestore:"topicId"   json:"topicId,omitempty"`
}

// BQParams is the BigQuery load-job settings stored on the document.
// NullMarker is nullable. SourceFormat is the integer 0 (CSV) or 1 (JSON).
type BQParams struct {
	FieldDelimiter  string       `firestore:"fieldDelimiter"  json:"fieldDelimiter,omitempty"`
	SkipLeadingRows int64        `firestore:"skipLeadingRows" json:"skipLeadingRows"`
	NullMarker      *string      `firestore:"nullMarker"      json:"nullMarker"`
	Quote           string       `firestore:"quote"           json:"quote,omitempty"`
	SourceFormat    SourceFormat `firestore:"sourceFormat"    json:"sourceFormat"`
}

// Mapping binds one BigQuery column to a CSV column or a SQL expression.
// isPartitionKey is not part of the domain struct; updates preserve it on the document.
type Mapping struct {
	Type                      MappingType `firestore:"type"                      json:"type"`
	Src                       string      `firestore:"src"                       json:"src"`
	PrimaryKey                bool        `firestore:"primaryKey"                json:"primaryKey"`
	UseInDeleteFilter         bool        `firestore:"useInDeleteFilter"         json:"useInDeleteFilter"`
	IsRequiredPartitionFilter bool        `firestore:"isRequiredPartitionFilter" json:"isRequiredPartitionFilter"`
}

// ListFilter is the query string of GET /api/v1/load-configs.
type ListFilter struct {
	PartnerType string
	ImportType  string
	Query       string
}

// PatternTest is the body of POST /api/v1/load-configs/test-pattern.
type PatternTest struct {
	Matches          bool    `json:"matches"`
	MatchingConfigID *string `json:"matchingConfigId"`
}

// Normalize trims human-entered text. Delimiter and quote are preserved as-is
// because a space or a tab is a meaningful delimiter.
func Normalize(cfg LoadConfig) LoadConfig {
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.PublisherName = strings.TrimSpace(cfg.PublisherName)
	cfg.Mode = LoadMode(strings.TrimSpace(string(cfg.Mode)))
	cfg.PartnerType = strings.ToLower(strings.TrimSpace(cfg.PartnerType))
	cfg.ImportType = strings.ToLower(strings.TrimSpace(cfg.ImportType))
	cfg.Identity = cfg.Normalized()
	cfg.Patterns.Preprocess = strings.TrimSpace(cfg.Patterns.Preprocess)
	cfg.Patterns.Ingest = strings.TrimSpace(cfg.Patterns.Ingest)
	cfg.Destination.ProjectID = strings.TrimSpace(cfg.Destination.ProjectID)
	cfg.Destination.DatasetID = strings.TrimSpace(cfg.Destination.DatasetID)
	cfg.Destination.TableID = strings.TrimSpace(cfg.Destination.TableID)
	cfg.Organization.ID = strings.TrimSpace(cfg.Organization.ID)
	cfg.Organization.Type = OrganizationType(strings.TrimSpace(string(cfg.Organization.Type)))
	cfg.Organization.Account = trimmedStringPtr(cfg.Organization.Account)
	cfg.Notification.ProjectID = strings.TrimSpace(cfg.Notification.ProjectID)
	cfg.Notification.TopicID = strings.TrimSpace(cfg.Notification.TopicID)
	cfg.BQParams.NullMarker = trimmedStringPtr(cfg.BQParams.NullMarker)

	if cfg.Mappings == nil {
		cfg.Mappings = map[string]Mapping{}
	}

	for key, mapping := range cfg.Mappings {
		mapping.Src = strings.TrimSpace(mapping.Src)
		cfg.Mappings[key] = mapping
	}

	return cfg
}

// Clone returns a deep copy so repository callers cannot mutate stored state.
func (c LoadConfig) Clone() LoadConfig {
	c.Organization.Account = cloneStringPtr(c.Organization.Account)
	c.BQParams.NullMarker = cloneStringPtr(c.BQParams.NullMarker)

	if c.Mappings != nil {
		c.Mappings = maps.Clone(c.Mappings)
	}

	return c
}

// HasPrimaryKey reports whether any mapping is flagged as a primary key.
func (c LoadConfig) HasPrimaryKey() bool {
	for _, mapping := range c.Mappings {
		if mapping.PrimaryKey {
			return true
		}
	}

	return false
}

// Present fills derived partner and import kinds for API responses.
// Convention documents also get the wizard inputs parsed from the id or patterns.
func (c LoadConfig) Present() LoadConfig {
	c.PartnerType, c.ImportType = Classify(c)

	c.Identity = Identity{}
	if parsed, ok := ParseIdentity(c); ok {
		c.Identity = parsed
	}

	return c
}

func trimmedStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)

	return &trimmed
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}

	copied := *value

	return &copied
}
