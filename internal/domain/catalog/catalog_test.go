package catalog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/catalog"
	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestTemplatesCoverProductMatrix(t *testing.T) {
	t.Parallel()

	items := catalog.Templates()
	require.Len(t, items, 6)

	byKind := map[string]model.Template{}
	for _, item := range items {
		byKind[item.PartnerType+"/"+item.ImportType] = item
		assert.Equal(t, item.PartnerType, item.Defaults.PartnerType)
		assert.Equal(t, item.ImportType, item.Defaults.ImportType)
		assert.Equal(t, model.ModeAppend, item.Defaults.Mode)
		assert.Equal(t, model.SourceFormatCSV, item.Defaults.BQParams.SourceFormat)
		assert.Equal(t, model.OrganizationType(item.PartnerType), item.Defaults.Organization.Type)
		require.NotEmpty(t, item.Columns)
		for _, column := range item.Columns {
			mapping, ok := item.Defaults.Mappings[column.Name]
			require.True(t, ok, column.Name)
			assert.Equal(t, column.Name, mapping.Src)
			assert.Equal(t, model.MappingTypeRename, mapping.Type)
		}
	}

	optin := byKind["publisher/optin"]
	assertRequired(t, optin, "mobile_phone", "optin_sms", "collect_date", "collect_url")
	assertColumnType(t, optin, "birth_date", model.BQDate)
	assertColumnType(t, optin, "email", model.BQString)

	customers := byKind["advertiser/customers"]
	assertRequired(t, customers, "mobile_phone", "optin_sms", "collect_date", "collect_url")
	assertColumnType(t, customers, "country", model.BQString)
	assertColumnType(t, customers, "additional_fields", model.BQJSON)
	for _, column := range optin.Columns {
		_, ok := customers.Defaults.Mappings[column.Name]
		assert.True(t, ok, column.Name)
	}

	sales := byKind["advertiser/sales"]
	assertRequired(t, sales, "order_ts", "mobile_phone", "store_id", "price_before_tax")
	assertColumnType(t, sales, "order_ts", model.BQTimestamp)
	assertColumnType(t, sales, "price_before_tax", model.BQFloat)
	assertColumnType(t, sales, "item_quantity", model.BQInteger)

	stores := byKind["advertiser/stores"]
	assertRequired(t, stores, "id", "name", "address", "zip_code", "city")
	assertColumnType(t, stores, "longitude", model.BQFloat)

	optout := byKind["publisher/optout"]
	assertRequired(t, optout, "sha256_mobile_phone")
	assertColumnType(t, optout, "sha256_mobile_phone", model.BQString)
	require.Len(t, optout.Columns, 1)

	blacklists := byKind["advertiser/blacklists"]
	assertRequired(t, blacklists, "sha256_mobile_phone")
	assertColumnType(t, blacklists, "sha256_mobile_phone", model.BQString)
	require.Len(t, blacklists.Columns, 1)
	assert.Equal(t, optout.Columns, blacklists.Columns)
	assert.Equal(t, "blacklists", blacklists.Defaults.Destination.TableID)
}

func TestMetaMappingTable(t *testing.T) {
	t.Parallel()

	meta := catalog.Meta()
	require.Len(t, meta.MappingTypes, 7)
	assert.Equal(t, model.MappingTypeRename, meta.MappingTypes[0].Value)
	assert.Equal(t, "RENAME", meta.MappingTypes[0].Label)
	assert.Equal(t, "ARRAY", meta.MappingTypes[6].Label)
	require.Len(t, meta.SourceFormats, 2)
	assert.Equal(t, model.SourceFormatCSV, meta.SourceFormats[0].Value)
	assert.Equal(t, "CSV", meta.SourceFormats[0].Label)
	assert.Equal(t, model.SourceFormatJSON, meta.SourceFormats[1].Value)
	assert.Equal(t, "JSON", meta.SourceFormats[1].Label)
	assert.Equal(t, []string{model.ImportOptin, model.ImportOptout}, meta.ImportTypes[model.PartnerPublisher])
	assert.Equal(t, []string{
		model.ImportBlacklists, model.ImportCustomers, model.ImportStores, model.ImportSales,
	}, meta.ImportTypes[model.PartnerAdvertiser])
}

func assertRequired(t *testing.T, item model.Template, names ...string) {
	t.Helper()

	required := map[string]bool{}
	for _, column := range item.Columns {
		if column.Required {
			required[column.Name] = true
		}
	}

	assert.Len(t, required, len(names))
	for _, name := range names {
		assert.True(t, required[name], name)
	}
}

func assertColumnType(t *testing.T, item model.Template, name, bqType string) {
	t.Helper()

	for _, column := range item.Columns {
		if column.Name == name {
			assert.Equal(t, bqType, column.BQType)

			return
		}
	}

	t.Fatalf("column %s not found", name)
}
