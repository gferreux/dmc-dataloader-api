package model

import (
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

// Source formats accepted on bqParams.sourceFormat (BigQuery load job values).
const (
	SourceFormatCSV  = "CSV"
	SourceFormatJSON = "NEWLINE_DELIMITED_JSON"
)

// DefaultProjectID is the dev GCP project used when a template needs a destination project.
const DefaultProjectID = "dmc-datastores-dev-becb"

// LoadMode is the Firestore mode string.
type LoadMode string

// Valid reports whether the mode is one the loader is known to accept.
func (m LoadMode) Valid() bool {
	switch m {
	case ModeAppend, ModeIncremental, ModeOverwrite:
		return true
	default:
		return false
	}
}

// Modes returns the mode values exposed by GET /api/v1/meta.
func Modes() []LoadMode {
	return []LoadMode{ModeAppend, ModeIncremental, ModeOverwrite}
}

// SourceFormats returns the source formats exposed by GET /api/v1/meta.
func SourceFormats() []string {
	return []string{SourceFormatCSV, SourceFormatJSON}
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

// LoadConfig is the API representation of a load_config document.
//
// JSON field names mirror the Firestore document. ID, CreateTime, and UpdateTime
// come from the document reference and snapshot metadata. PartnerType and ImportType
// are derived for the console and are tagged firestore:"-" so they are not written.
type LoadConfig struct {
	ID          string    `firestore:"-" json:"id,omitempty"`
	CreateTime  time.Time `firestore:"-" json:"createTime,omitzero"`
	UpdateTime  time.Time `firestore:"-" json:"updateTime,omitzero"`
	PartnerType string    `firestore:"-" json:"partnerType,omitempty"`
	ImportType  string    `firestore:"-" json:"importType,omitempty"`

	PublisherName string             `firestore:"publisherName"          json:"publisherName"`
	Mode          LoadMode           `firestore:"mode"                   json:"mode"`
	Deactivated   bool               `firestore:"deactivated"            json:"deactivated"`
	Incremental   *bool              `firestore:"incremental,omitempty"  json:"incremental,omitempty"`
	Patterns      Patterns           `firestore:"patterns"               json:"patterns"`
	Destination   Destination        `firestore:"destination"            json:"destination"`
	Organization  *Organization      `firestore:"organization,omitempty" json:"organization,omitempty"`
	Notification  *Notification      `firestore:"notification,omitempty" json:"notification,omitempty"`
	BQParams      BQParams           `firestore:"bqParams"               json:"bqParams"`
	Mappings      map[string]Mapping `firestore:"mappings"               json:"mappings"`
}

// Patterns holds the regexes the loader matches against bucket/objectName.
type Patterns struct {
	Preprocess string `firestore:"preprocess,omitempty" json:"preprocess,omitempty"`
	Ingest     string `firestore:"ingest,omitempty"     json:"ingest,omitempty"`
}

// Destination is the BigQuery table that receives the file.
type Destination struct {
	ProjectID string `firestore:"projectId" json:"projectId"`
	DatasetID string `firestore:"datasetId" json:"datasetId"`
	TableID   string `firestore:"tableId"   json:"tableId"`
}

// Organization identifies the partner account on the document.
// Type is stored as the loader left it. It is only treated as a partner kind
// when the value is exactly "publisher" or "advertiser".
type Organization struct {
	ID      string `firestore:"id"      json:"id"`
	Account string `firestore:"account" json:"account"`
	Type    string `firestore:"type"    json:"type"`
}

// Notification is the Pub/Sub topic notified after a load.
type Notification struct {
	ProjectID string `firestore:"projectId" json:"projectId"`
	TopicID   string `firestore:"topicId"   json:"topicId"`
}

// BQParams is the subset of BigQuery load-job settings stored on the document.
type BQParams struct {
	FieldDelimiter  string `firestore:"fieldDelimiter,omitempty" json:"fieldDelimiter,omitempty"`
	SkipLeadingRows int    `firestore:"skipLeadingRows"          json:"skipLeadingRows"`
	NullMarker      string `firestore:"nullMarker,omitempty"     json:"nullMarker,omitempty"`
	Quote           string `firestore:"quote,omitempty"          json:"quote,omitempty"`
	SourceFormat    string `firestore:"sourceFormat"             json:"sourceFormat"`
}

// Mapping binds one BigQuery column to a CSV column or a SQL expression.
type Mapping struct {
	Src                       string       `firestore:"src"                                 json:"src"`
	Type                      *MappingType `firestore:"type"                                json:"type"`
	PrimaryKey                bool         `firestore:"primaryKey,omitempty"                json:"primaryKey,omitempty"`
	IsPartitionKey            bool         `firestore:"isPartitionKey,omitempty"            json:"isPartitionKey,omitempty"`            //nolint:lll // tag alignment exceeds the line limit
	UseInDeleteFilter         bool         `firestore:"useInDeleteFilter,omitempty"         json:"useInDeleteFilter,omitempty"`         //nolint:lll // tag alignment exceeds the line limit
	IsRequiredPartitionFilter bool         `firestore:"isRequiredPartitionFilter,omitempty" json:"isRequiredPartitionFilter,omitempty"` //nolint:lll // tag alignment exceeds the line limit
}

// ListFilter is the query string of GET /api/v1/load-configs.
type ListFilter struct {
	PartnerType        string
	ImportType         string
	Query              string
	IncludeDeactivated bool
}

// PatternTest is the body of POST /api/v1/load-configs/test-pattern.
type PatternTest struct {
	Matches          bool    `json:"matches"`
	MatchingConfigID *string `json:"matchingConfigId"`
}

// BoolPtr returns a pointer to a copy of v.
func BoolPtr(v bool) *bool {
	copied := v

	return &copied
}

// Normalize trims human-entered text. Delimiter and quote are preserved as-is
// because a space or a tab is a meaningful delimiter.
func Normalize(cfg LoadConfig) LoadConfig {
	cfg.ID = strings.TrimSpace(cfg.ID)
	cfg.PublisherName = strings.TrimSpace(cfg.PublisherName)
	cfg.Mode = LoadMode(strings.TrimSpace(string(cfg.Mode)))
	cfg.PartnerType = strings.ToLower(strings.TrimSpace(cfg.PartnerType))
	cfg.ImportType = strings.ToLower(strings.TrimSpace(cfg.ImportType))
	cfg.Patterns.Preprocess = strings.TrimSpace(cfg.Patterns.Preprocess)
	cfg.Patterns.Ingest = strings.TrimSpace(cfg.Patterns.Ingest)
	cfg.Destination.ProjectID = strings.TrimSpace(cfg.Destination.ProjectID)
	cfg.Destination.DatasetID = strings.TrimSpace(cfg.Destination.DatasetID)
	cfg.Destination.TableID = strings.TrimSpace(cfg.Destination.TableID)
	cfg.BQParams.SourceFormat = strings.TrimSpace(cfg.BQParams.SourceFormat)

	if cfg.Organization != nil {
		org := *cfg.Organization
		org.ID = strings.TrimSpace(org.ID)
		org.Account = strings.TrimSpace(org.Account)
		org.Type = strings.TrimSpace(org.Type)
		cfg.Organization = &org
	}

	if cfg.Notification != nil {
		note := *cfg.Notification
		note.ProjectID = strings.TrimSpace(note.ProjectID)
		note.TopicID = strings.TrimSpace(note.TopicID)
		cfg.Notification = &note
	}

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
	if c.Incremental != nil {
		c.Incremental = BoolPtr(*c.Incremental)
	}

	if c.Organization != nil {
		org := *c.Organization
		c.Organization = &org
	}

	if c.Notification != nil {
		note := *c.Notification
		c.Notification = &note
	}

	if c.Mappings != nil {
		copied := make(map[string]Mapping, len(c.Mappings))
		for key, mapping := range c.Mappings {
			if mapping.Type != nil {
				mapping.Type = MappingTypePtr(*mapping.Type)
			}

			copied[key] = mapping
		}

		c.Mappings = copied
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
func (c LoadConfig) Present() LoadConfig {
	c.PartnerType, c.ImportType = Classify(c)

	return c
}
