package model_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestMappingTypesAreZeroThroughSix(t *testing.T) {
	t.Parallel()

	infos := model.MappingTypes()
	require.Len(t, infos, 7)
	for i, info := range infos {
		assert.Equal(t, model.MappingType(i), info.Value)
		assert.True(t, info.Value.Valid())
		assert.Equal(t, info.BQType, info.Label)
		parsed, ok := model.MappingTypeFromBQ(info.BQType)
		assert.True(t, ok)
		assert.Equal(t, info.Value, parsed)
	}

	assert.False(t, model.MappingType(7).Valid())
}

func TestClassify(t *testing.T) {
	t.Parallel()

	partner, importType := model.Classify(model.LoadConfig{
		ID: "acme:demo:sales",
		Destination: model.Destination{
			DatasetID: "dkp_dmc_advertisers_raw_eu_dev",
			TableID:   "sales",
		},
	})
	assert.Equal(t, model.PartnerAdvertiser, partner)
	assert.Equal(t, model.ImportSales, importType)

	partner, importType = model.Classify(model.LoadConfig{
		Destination: model.Destination{
			DatasetID: "dkp_dmc_publishers_raw_eu_dev",
			TableID:   "profiles",
		},
	})
	assert.Equal(t, model.PartnerPublisher, partner)
	assert.Equal(t, model.ImportOptin, importType)

	partner, importType = model.Classify(model.LoadConfig{
		Organization: &model.Organization{Type: "retail"},
		Destination:  model.Destination{TableID: "fr_robinson"},
	})
	assert.Empty(t, partner)
	assert.Empty(t, importType)
}

func TestDerivedFieldsAreNotPersisted(t *testing.T) {
	t.Parallel()

	typ := reflect.TypeOf(model.LoadConfig{})
	for _, name := range []string{"ID", "CreateTime", "UpdateTime", "PartnerType", "ImportType"} {
		field, ok := typ.FieldByName(name)
		require.True(t, ok, name)
		assert.Equal(t, "-", field.Tag.Get("firestore"), name)
	}
}

func TestFirestoreTagsMatchJSONNames(t *testing.T) {
	t.Parallel()

	assertTags(t, reflect.TypeOf(model.LoadConfig{}))
}

func assertTags(t *testing.T, typ reflect.Type) {
	t.Helper()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
		fsName := strings.Split(field.Tag.Get("firestore"), ",")[0]
		if jsonName == "" || jsonName == "-" || fsName == "-" {
			continue
		}

		assert.Equal(t, jsonName, fsName, field.Name)

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Map {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct && fieldType.PkgPath() == typ.PkgPath() {
			assertTags(t, fieldType)
		}
	}
}
