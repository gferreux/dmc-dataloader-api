// Package catalog builds the import templates and the meta payload.
package catalog

import "github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"

const (
	datasetPublishers  = "dkp_dmc_publishers_raw_eu_dev"
	datasetAdvertisers = "dkp_dmc_advertisers_raw_eu_dev"

	suggestedIngest = `^REPLACE_BUCKET/REPLACE_PREFIX/.*\.csv$`

	hintMobile = "Preferred +33612345678. Also 0612345678 or 612345678 " +
		"with a default dial code."
	hintOptin  = "0/1 or true/false."
	hintGender = "M/F or m./mr/mme/mlle/mll."
	hintDate   = "YYYY-MM-DD preferred. Also YYYY/MM/DD, DD/MM/YYYY, DD-MM-YYYY."
	hintJSON   = "JSON object of extra key/values used for reporting drilldown."
	hintTags   = "Comma-separated tags."
)

// Templates returns one recommended starter document per partner and import kind.
func Templates() []model.Template {
	return []model.Template{
		publisherOptin(),
		publisherOptout(),
		advertiserBlacklists(),
		advertiserCustomers(),
		advertiserStores(),
		advertiserSales(),
	}
}

// Meta returns the closed sets the console uses to render forms.
func Meta() model.Meta {
	return model.Meta{
		Modes:         model.Modes(),
		MappingTypes:  model.MappingTypes(),
		SourceFormats: model.SourceFormats(),
		PartnerTypes:  model.PartnerTypes(),
		ImportTypes:   model.ImportTypes(),
	}
}

func col(name, bqType, description, formatHint string, required bool) model.TemplateColumn {
	return model.TemplateColumn{
		Name:        name,
		BQType:      bqType,
		Required:    required,
		Description: description,
		FormatHint:  formatHint,
	}
}

func defaults(
	partner string,
	importType string,
	dataset string,
	table string,
	columns []model.TemplateColumn,
) model.LoadConfig {
	return model.LoadConfig{
		PartnerType: partner,
		ImportType:  importType,
		Mode:        model.ModeAppend,
		Patterns:    model.Patterns{Ingest: suggestedIngest},
		Destination: model.Destination{
			ProjectID: model.DefaultProjectID,
			DatasetID: dataset,
			TableID:   table,
		},
		BQParams: model.BQParams{
			FieldDelimiter:  ",",
			SkipLeadingRows: 1,
			Quote:           `"`,
			SourceFormat:    model.SourceFormatCSV,
		},
		Mappings: mappingsFor(columns),
	}
}

func mappingsFor(columns []model.TemplateColumn) map[string]model.Mapping {
	mappings := make(map[string]model.Mapping, len(columns))
	for _, column := range columns {
		mappingType, ok := model.MappingTypeFromBQ(column.BQType)
		if !ok {
			continue
		}

		mappings[column.Name] = model.Mapping{
			Src:  column.Name,
			Type: model.MappingTypePtr(mappingType),
		}
	}

	return mappings
}
