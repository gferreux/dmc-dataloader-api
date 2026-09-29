package model

// Template is a recommended starting document for one partner and import kind.
type Template struct {
	PartnerType       string           `json:"partnerType"`
	ImportType        string           `json:"importType"`
	Label             string           `json:"label"`
	NeedsConfirmation bool             `json:"needsConfirmation"`
	ConfirmationNote  string           `json:"confirmationNote,omitempty"`
	Columns           []TemplateColumn `json:"columns"`
	Defaults          LoadConfig       `json:"defaults"`
}

// TemplateColumn describes one CSV / BigQuery column in a template.
type TemplateColumn struct {
	Name        string `json:"name"`
	BQType      string `json:"bqType"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
	FormatHint  string `json:"formatHint,omitempty"`
}

// Meta is the body of GET /api/v1/meta.
type Meta struct {
	Modes         []LoadMode          `json:"modes"`
	MappingTypes  []MappingTypeInfo   `json:"mappingTypes"`
	SourceFormats []string            `json:"sourceFormats"`
	PartnerTypes  []string            `json:"partnerTypes"`
	ImportTypes   map[string][]string `json:"importTypes"`
}
