package loadconfig

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func review(cfg model.LoadConfig, others []model.LoadConfig) model.ValidationReport {
	report := model.EmptyReport()
	check := &checker{report: report}
	check.identity(cfg)
	check.organization(cfg)
	check.patterns(cfg)
	check.destination(cfg)
	check.params(cfg)
	check.mappings(cfg)
	check.notification(cfg)
	check.classification(cfg)
	check.warnings(cfg, others)

	report = check.report
	if report.Errors == nil {
		report.Errors = []model.FieldIssue{}
	}

	if report.Warnings == nil {
		report.Warnings = []model.FieldIssue{}
	}

	return report
}

type checker struct {
	report model.ValidationReport
}

func (c *checker) error(field, message string) {
	c.report.Errors = append(c.report.Errors, model.FieldIssue{Field: field, Message: message})
}

func (c *checker) warn(field, message string) {
	c.report.Warnings = append(c.report.Warnings, model.FieldIssue{Field: field, Message: message})
}

func (c *checker) identity(cfg model.LoadConfig) {
	if cfg.ID == "" {
		c.error("id", "id is required")
	} else if !validDocumentID(cfg.ID) {
		c.error("id", "id must be 1-200 characters of letters, digits, '_', '.', ':' or '-'")
	}

	if cfg.PublisherName == "" {
		c.error("publisherName", "publisherName is required")
	}

	if !cfg.Mode.Valid() {
		c.error("mode", "mode must be APPEND, INCREMENTAL, or OVERWRITE")
	}
}

func validDocumentID(id string) bool {
	if utf8.RuneCountInString(id) > maxIDLen {
		return false
	}

	matched, err := regexp.MatchString(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`, id)

	return err == nil && matched
}

func (c *checker) patterns(cfg model.LoadConfig) {
	if cfg.Patterns.Preprocess == "" && cfg.Patterns.Ingest == "" {
		c.error("patterns", "at least one of patterns.preprocess or patterns.ingest is required")

		return
	}

	c.compileField("patterns.preprocess", cfg.Patterns.Preprocess)
	c.compileField("patterns.ingest", cfg.Patterns.Ingest)
}

func (c *checker) compileField(field, pattern string) {
	if pattern == "" {
		return
	}

	if _, err := compilePattern(pattern); err != nil {
		c.error(field, "must be a valid regex: "+err.Error())
	}
}

func (c *checker) destination(cfg model.LoadConfig) {
	if cfg.Destination.ProjectID == "" {
		c.error("destination.projectId", "destination.projectId is required")
	}

	if cfg.Destination.DatasetID == "" {
		c.error("destination.datasetId", "destination.datasetId is required")
	}

	if cfg.Destination.TableID == "" {
		c.error("destination.tableId", "destination.tableId is required")
	}
}

func (c *checker) organization(cfg model.LoadConfig) {
	if cfg.Organization.Type.Valid() {
		return
	}

	c.error("organization.type", "organization.type must be advertiser or publisher")
}

func (c *checker) params(cfg model.LoadConfig) {
	if cfg.BQParams.SourceFormat.Valid() {
		return
	}

	c.error("bqParams.sourceFormat", "sourceFormat must be 0 (CSV) or 1 (JSON)")
}

func (c *checker) mappings(cfg model.LoadConfig) {
	for name, mapping := range cfg.Mappings {
		field := "mappings." + name
		if strings.TrimSpace(name) == "" {
			c.error("mappings", "mapping column name is required")
		}

		if mapping.Src == "" {
			c.error(field+".src", "src is required")
		}

		if !mapping.Type.Valid() {
			c.error(field+".type", "type must be 0 RENAME, 1 SQL, 2 PREFIX_PATTERN, 3 CUSTOM, "+
				"4 EXTRA_FIELDS, 5 MISSING_MAPPINGS, or 6 ARRAY")
		}
	}
}

func (c *checker) notification(cfg model.LoadConfig) {
	hasProject := cfg.Notification.ProjectID != ""

	hasTopic := cfg.Notification.TopicID != ""
	if hasProject != hasTopic {
		c.error("notification", "notification.projectId and notification.topicId must both be set")
	}
}

func (c *checker) classification(cfg model.LoadConfig) {
	partner, importType := model.Classify(cfg)
	if cfg.PartnerType != "" && !model.ValidPartner(cfg.PartnerType) {
		c.error("partnerType", "partnerType must be publisher or advertiser")
	}

	if cfg.ImportType != "" && !model.KnownImport(cfg.ImportType) {
		c.error("importType", "importType is not a known import kind")
	}

	if partner == "" {
		c.error("partnerType",
			"partnerType cannot be derived; set organization.type to publisher or advertiser, "+
				"or use a dataset id that contains publishers or advertisers")
	}

	if importType == "" {
		c.error("importType",
			"importType cannot be derived; end the document id with :optin, :optout, :blacklists, "+
				":customers, :stores, or :sales, or use a known destination.tableId")
	}

	if partner != "" && importType != "" && !model.ValidImport(partner, importType) {
		c.error("importType", "importType is not valid for the derived partnerType")
	}

	if cfg.PartnerType != "" && partner != "" && cfg.PartnerType != partner {
		c.error("partnerType", "partnerType does not match the value derived from the document")
	}

	if cfg.ImportType != "" && importType != "" && cfg.ImportType != importType {
		c.error("importType", "importType does not match the value derived from the document")
	}
}
