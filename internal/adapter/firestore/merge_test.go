package firestore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dekuple-labs/dmc-dataloader-api/internal/domain/model"
)

func TestDocumentDataStoresSourceFormatAsInt(t *testing.T) {
	t.Parallel()

	data := documentData(model.LoadConfig{
		PublisherName: "Acme",
		Mode:          model.ModeAppend,
		BQParams:      model.BQParams{SourceFormat: model.SourceFormatCSV, SkipLeadingRows: 1},
		Mappings: map[string]model.Mapping{
			"email": {Src: "email", Type: model.MappingTypeRename},
		},
		Organization: model.Organization{Type: model.OrganizationTypeAdvertiser},
	})

	_, hasDeactivated := data["deactivated"]
	assert.False(t, hasDeactivated)

	params, ok := data["bqParams"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(0), params["sourceFormat"])
	assert.Nil(t, params["nullMarker"])

	organization, ok := data["organization"].(map[string]any)
	require.True(t, ok)
	assert.Nil(t, organization["account"])

	mappings, ok := data["mappings"].(map[string]any)
	require.True(t, ok)
	email, ok := mappings["email"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, int64(0), email["type"])
	_, hasPartition := email["isPartitionKey"]
	assert.False(t, hasPartition)
}

func TestMergePreservingUnknown(t *testing.T) {
	t.Parallel()

	existing := map[string]any{
		"publisherName": "Old",
		"deactivated":   true,
		"incremental":   false,
		"legacyNote":    "keep",
		"mode":          "APPEND",
		"mappings": map[string]any{
			"email": map[string]any{
				"src":            "email",
				"type":           int64(0),
				"isPartitionKey": true,
				"primaryKey":     true,
			},
			"removed": map[string]any{"src": "removed", "type": int64(0)},
		},
		"bqParams": map[string]any{
			"sourceFormat": int64(0),
			"customFlag":   "stay",
		},
	}
	incoming := documentData(model.LoadConfig{
		PublisherName: "New",
		Mode:          model.ModeAppend,
		BQParams:      model.BQParams{SourceFormat: model.SourceFormatJSON, SkipLeadingRows: 2},
		Mappings: map[string]model.Mapping{
			"email": {Src: "mail", Type: model.MappingTypeSQL},
			"added": {Src: "added", Type: model.MappingTypeRename},
		},
		Organization: model.Organization{Type: model.OrganizationTypePublisher},
	})

	merged := mergePreservingUnknown(existing, incoming)

	assert.Equal(t, "New", merged["publisherName"])
	assert.Equal(t, true, merged["deactivated"])
	assert.Equal(t, false, merged["incremental"])
	assert.Equal(t, "keep", merged["legacyNote"])
	assert.Equal(t, "email", existing["mappings"].(map[string]any)["email"].(map[string]any)["src"])

	mappings, ok := merged["mappings"].(map[string]any)
	require.True(t, ok)
	_, removed := mappings["removed"]
	assert.False(t, removed)
	_, added := mappings["added"]
	assert.True(t, added)

	email, ok := mappings["email"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "mail", email["src"])
	assert.Equal(t, int64(model.MappingTypeSQL), email["type"])
	assert.Equal(t, true, email["isPartitionKey"])
	assert.Equal(t, false, email["primaryKey"])

	params, ok := merged["bqParams"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "stay", params["customFlag"])
	assert.Equal(t, int64(model.SourceFormatJSON), params["sourceFormat"])
	assert.Equal(t, int64(2), params["skipLeadingRows"])
}
