package model

import (
	"maps"
	"regexp"
)

// DeriveConfig holds the per-kind buckets, notification topic, and BigQuery destination
// used to build a new load_config. Prod overrides every value through viper.
type DeriveConfig struct {
	Advertiser PartnerDeriveConfig `mapstructure:"advertiser" validate:"required"`
	Publisher  PartnerDeriveConfig `mapstructure:"publisher"  validate:"required"`
}

// PartnerDeriveConfig is the plumbing for one organization kind.
type PartnerDeriveConfig struct {
	RawBucket     string               `mapstructure:"raw_bucket"     validate:"required"`
	StagingBucket string               `mapstructure:"staging_bucket" validate:"required"`
	Notification  NotificationSettings `mapstructure:"notification"   validate:"required"`
	Destination   DestinationSettings  `mapstructure:"destination"    validate:"required"`
}

// NotificationSettings is the Pub/Sub topic notified after a load.
type NotificationSettings struct {
	ProjectID string `mapstructure:"project_id" validate:"required"`
	TopicID   string `mapstructure:"topic_id"   validate:"required"`
}

// DestinationSettings is the BigQuery table prefix for one kind.
// Tables is the publisher map (optin → profiles, optout → optout).
// Advertiser tables are the file type and ignore this map.
type DestinationSettings struct {
	ProjectID string            `mapstructure:"project_id" validate:"required"`
	DatasetID string            `mapstructure:"dataset_id" validate:"required"`
	Tables    map[string]string `mapstructure:"tables"`
}

const (
	extPattern = `[.](csv|zip|gz|gzip|tgz|tar\.gz|7z)`
	tsPattern  = `[0-9]{4}-[01][0-9]-[0-3][0-9]T[0-2][0-9]:[0-5][0-9]:[0-5][0-9]Z`

	defaultAdvertiserRawBucket     = "dkp-dmc-advertisers-raw-euw1-dev"
	defaultAdvertiserStagingBucket = "dkp-dmc-advertisers-staging-euw1-dev"
	defaultPublisherRawBucket      = "dkp-dmc-publishers-raw-euw1-dev"
	defaultPublisherStagingBucket  = "dkp-dmc-publishers-staging-euw1-dev"
	defaultNotificationProject     = "dmc-curated-inventory-dev-e6da"
	defaultNotificationTopic       = "dkp-dmc-data-loader-notifications-dev"
	defaultAdvertiserProject       = "dmc-raw-advertisers-dev-27c7"
	defaultAdvertiserDataset       = "dkp_dmc_advertisers_raw_eu_dev"
	defaultPublisherProject        = "dmc-raw-publishers-dev-c69c"
	defaultPublisherDataset        = "dkp_dmc_publishers_raw_eu_dev"
	defaultOrganizationProject     = "dmc-datastores-dev-becb"
	defaultOrganizationDatabase    = "(default)"
)

// DevDeriveConfig is the dev plumbing used when no config file or env var overrides it.
func DevDeriveConfig() DeriveConfig {
	notification := NotificationSettings{
		ProjectID: defaultNotificationProject,
		TopicID:   defaultNotificationTopic,
	}

	return DeriveConfig{
		Advertiser: PartnerDeriveConfig{
			RawBucket:     defaultAdvertiserRawBucket,
			StagingBucket: defaultAdvertiserStagingBucket,
			Notification:  notification,
			Destination: DestinationSettings{
				ProjectID: defaultAdvertiserProject,
				DatasetID: defaultAdvertiserDataset,
			},
		},
		Publisher: PartnerDeriveConfig{
			RawBucket:     defaultPublisherRawBucket,
			StagingBucket: defaultPublisherStagingBucket,
			Notification:  notification,
			Destination: DestinationSettings{
				ProjectID: defaultPublisherProject,
				DatasetID: defaultPublisherDataset,
				Tables: map[string]string{
					ImportOptin:  "profiles",
					ImportOptout: "optout",
				},
			},
		},
	}
}

// DevOrganizationSource is the dev organization database used when nothing overrides it.
func DevOrganizationSource() OrganizationSourceConfig {
	return OrganizationSourceConfig{
		ProjectID:          defaultOrganizationProject,
		DatabaseID:         defaultOrganizationDatabase,
		Collection:         "organizations",
		AccountsCollection: "accounts",
	}
}

// WithTableDefaults fills publisher table names that were left empty.
func (c DeriveConfig) WithTableDefaults() DeriveConfig {
	tables := make(map[string]string, len(c.Publisher.Destination.Tables)+2)
	maps.Copy(tables, c.Publisher.Destination.Tables)

	if tables[ImportOptin] == "" {
		tables[ImportOptin] = "profiles"
	}

	if tables[ImportOptout] == "" {
		tables[ImportOptout] = "optout"
	}

	c.Publisher.Destination.Tables = tables

	return c
}

// BuildDerived fills every server-owned load_config field except organization ids.
// Publisher organization.account is the empty string. Advertiser account is left nil
// until the organization directory resolves it.
func BuildDerived(identity Identity, settings DeriveConfig) (LoadConfig, error) {
	identity = identity.Normalized()
	if issues := identity.Issues(); len(issues) > 0 {
		return LoadConfig{}, &ValidationError{Issues: issues}
	}

	orgSlug, err := Slug(identity.OrganizationName)
	if err != nil {
		return LoadConfig{}, err
	}

	nestedSlug, err := Slug(identity.NestedName)
	if err != nil {
		return LoadConfig{}, err
	}

	partner := settings.Advertiser
	if identity.Kind == PartnerPublisher {
		partner = settings.Publisher
	}

	tableID, err := partner.tableID(identity)
	if err != nil {
		return LoadConfig{}, err
	}

	key := orgSlug + ":" + nestedSlug + ":" + identity.FileType

	cfg := LoadConfig{
		ID:            key,
		PublisherName: key,
		Patterns: Patterns{
			Preprocess: preprocessPattern(partner.RawBucket, orgSlug, nestedSlug, identity.FileType),
			Ingest:     ingestPattern(partner.StagingBucket, orgSlug, nestedSlug, identity.FileType),
		},
		Destination: Destination{
			ProjectID: partner.Destination.ProjectID,
			DatasetID: partner.Destination.DatasetID,
			TableID:   tableID,
		},
		Notification: Notification{
			ProjectID: partner.Notification.ProjectID,
			TopicID:   partner.Notification.TopicID,
		},
		Organization: Organization{Type: OrganizationType(identity.Kind)},
		Identity: Identity{
			Kind:             identity.Kind,
			OrganizationName: orgSlug,
			NestedName:       nestedSlug,
			FileType:         identity.FileType,
		},
	}
	if identity.Kind == PartnerPublisher {
		empty := ""
		cfg.Organization.Account = &empty
	}

	return cfg, nil
}

func (c PartnerDeriveConfig) tableID(identity Identity) (string, error) {
	if identity.Kind == PartnerAdvertiser {
		return identity.FileType, nil
	}

	tableID := c.Destination.Tables[identity.FileType]
	if tableID == "" {
		return "", &ValidationError{Issues: []FieldIssue{{
			Field:   "destination.tableId",
			Message: "no destination table configured for " + identity.FileType,
		}}}
	}

	return tableID, nil
}

func preprocessPattern(bucket, orgSlug, nestedSlug, fileType string) string {
	prefix := bucket + "/" + orgSlug + "/" + nestedSlug + "/" + fileType + "/"

	return "^" + regexp.QuoteMeta(prefix) + ".+" + extPattern + "$"
}

func ingestPattern(bucket, orgSlug, nestedSlug, fileType string) string {
	head := bucket + "/data/"
	tail := "/" + orgSlug + "/" + nestedSlug + "/" + fileType + "/"

	return "^" + regexp.QuoteMeta(head) + tsPattern + regexp.QuoteMeta(tail) + ".+$"
}
