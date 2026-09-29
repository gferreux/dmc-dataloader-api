package firestore

import "github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"

// documentData is the Firestore body for the fields LoadConfig models.
// sourceFormat and mapping type are integers, matching the domain model.
// Nullable account and nullMarker are written as null when unset.
func documentData(cfg model.LoadConfig) map[string]any {
	return map[string]any{
		"publisherName": cfg.PublisherName,
		"mode":          string(cfg.Mode),
		"patterns": map[string]any{
			"preprocess": cfg.Patterns.Preprocess,
			"ingest":     cfg.Patterns.Ingest,
		},
		"destination": map[string]any{
			"projectId": cfg.Destination.ProjectID,
			"datasetId": cfg.Destination.DatasetID,
			"tableId":   cfg.Destination.TableID,
		},
		"bqParams": map[string]any{
			"fieldDelimiter":  cfg.BQParams.FieldDelimiter,
			"skipLeadingRows": cfg.BQParams.SkipLeadingRows,
			"nullMarker":      stringOrNil(cfg.BQParams.NullMarker),
			"quote":           cfg.BQParams.Quote,
			"sourceFormat":    int64(cfg.BQParams.SourceFormat),
		},
		"mappings":     mappingsData(cfg.Mappings),
		"organization": organizationData(cfg.Organization),
		"notification": map[string]any{
			"projectId": cfg.Notification.ProjectID,
			"topicId":   cfg.Notification.TopicID,
		},
	}
}

func organizationData(org model.Organization) map[string]any {
	return map[string]any{
		"id":      org.ID,
		"account": stringOrNil(org.Account),
		"type":    string(org.Type),
	}
}

func mappingsData(mappings map[string]model.Mapping) map[string]any {
	if mappings == nil {
		mappings = map[string]model.Mapping{}
	}

	encoded := make(map[string]any, len(mappings))
	for name, mapping := range mappings {
		encoded[name] = map[string]any{
			"type":                      int64(mapping.Type),
			"src":                       mapping.Src,
			"primaryKey":                mapping.PrimaryKey,
			"useInDeleteFilter":         mapping.UseInDeleteFilter,
			"isRequiredPartitionFilter": mapping.IsRequiredPartitionFilter,
		}
	}

	return encoded
}

func stringOrNil(value *string) any {
	if value == nil {
		return nil
	}

	return *value
}
