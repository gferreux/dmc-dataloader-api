package model

import "strings"

// Classify derives partnerType and importType from fields the loader already stores.
//
// partnerType comes from organization.type when that value is publisher or advertiser,
// otherwise from destination.datasetId (publishers / advertisers), otherwise from the
// import type. importType prefers the last colon-separated segment of the document id
// when it is a known kind, otherwise destination.tableId.
//
// profiles is treated as publisher opt-in. customers and stores table ids are assumed
// to match the import kind. The result can be empty when the document does not carry
// enough information. Contradictions are left intact for validation to report.
func Classify(cfg LoadConfig) (string, string) {
	partner := partnerFromOrganization(cfg.Organization)
	if partner == "" {
		partner = partnerFromDataset(cfg.Destination.DatasetID)
	}

	importType := importFromID(cfg.ID)
	if importType == "" {
		importType = importFromTable(cfg.Destination.TableID)
	}

	if partner == "" {
		partner, _ = PartnerForImport(importType)
	}

	return partner, importType
}

func partnerFromOrganization(org *Organization) string {
	if org == nil {
		return ""
	}

	switch strings.ToLower(org.Type) {
	case PartnerPublisher:
		return PartnerPublisher
	case PartnerAdvertiser:
		return PartnerAdvertiser
	default:
		return ""
	}
}

func partnerFromDataset(datasetID string) string {
	lowered := strings.ToLower(datasetID)
	hasPublisher := strings.Contains(lowered, "publisher")
	hasAdvertiser := strings.Contains(lowered, "advertiser")

	switch {
	case hasPublisher && hasAdvertiser:
		return ""
	case hasPublisher:
		return PartnerPublisher
	case hasAdvertiser:
		return PartnerAdvertiser
	default:
		return ""
	}
}

func importFromID(id string) string {
	if id == "" {
		return ""
	}

	parts := strings.Split(id, ":")

	last := strings.ToLower(parts[len(parts)-1])
	if KnownImport(last) {
		return last
	}

	return ""
}

func importFromTable(tableID string) string {
	switch strings.ToLower(tableID) {
	case "profiles", "profile", "optin":
		return ImportOptin
	case "optout":
		return ImportOptout
	case "blacklists", "blacklist":
		return ImportBlacklists
	case "customers", "customer":
		return ImportCustomers
	case "stores", "store":
		return ImportStores
	case "sales", "sale":
		return ImportSales
	default:
		return ""
	}
}
