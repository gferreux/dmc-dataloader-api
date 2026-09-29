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
		require.NotEmpty(t, item.Columns)
		for _, column := range item.Columns {
			mapping, ok := item.Defaults.Mappings[column.Name]
			require.True(t, ok, column.Name)
			assert.Equal(t, column.Name, mapping.Src)
			require.NotNil(t, mapping.Type)
			expected, ok := model.MappingTypeFromBQ(column.BQType)
			require.True(t, ok, column.BQType)
			assert.Equal(t, expected, *mapping.Type)
		}
	}

	optin := byKind["publisher/optin"]
	assert.False(t, optin.NeedsConfirmation)
	assertRequired(t, optin, "mobile_phone", "optin_sms", "collect_date", "collect_url")
	assertColumnType(t, optin, "birth_date", model.BQDate)
	assertColumnType(t, optin, "email", model.BQString)

	customers := byKind["advertiser/customers"]
	assert.False(t, customers.NeedsConfirmation)
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
	assert.True(t, optout.NeedsConfirmation)
	assert.NotEmpty(t, optout.ConfirmationNote)
	assertRequired(t, optout, "mobile_phone", "optout_date")

	blacklists := byKind["advertiser/blacklists"]
	assert.True(t, blacklists.NeedsConfirmation)
	assert.NotEmpty(t, blacklists.ConfirmationNote)
	assertRequired(t, blacklists, "mobile_phone")
	assert.Equal(t, "blacklists", blacklists.Defaults.Destination.TableID)
}

func TestMetaMappingTable(t *testing.T) {
	t.Parallel()

	meta := catalog.Meta()
	require.Len(t, meta.MappingTypes, 7)
	assert.Equal(t, model.MappingTypeString, meta.MappingTypes[0].Value)
	assert.Equal(t, model.BQJSON, meta.MappingTypes[6].Label)
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
