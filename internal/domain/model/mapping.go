package model

// BigQuery type names used by CSV templates. They are not mapping-type values.
const (
	BQString    = "STRING"
	BQInteger   = "INTEGER"
	BQFloat     = "FLOAT"
	BQBoolean   = "BOOLEAN"
	BQTimestamp = "TIMESTAMP"
	BQDate      = "DATE"
	BQJSON      = "JSON"
)

// Mapping types are the iota in
// dekuple-labs/dmc-domain/pkg/model/data_loader_config.go.
// 7 is ND (undefined) and is not a valid stored value.
const (
	MappingTypeRename          MappingType = 0
	MappingTypeSQL             MappingType = 1
	MappingTypePrefixPattern   MappingType = 2
	MappingTypeCustom          MappingType = 3
	MappingTypeExtraFields     MappingType = 4
	MappingTypeMissingMappings MappingType = 5
	MappingTypeArray           MappingType = 6
)

// MappingType is the Firestore integer stored on mappings.<column>.type.
type MappingType int

// MappingTypeInfo is one entry of GET /api/v1/meta mappingTypes.
type MappingTypeInfo struct {
	Value MappingType `json:"value"`
	Label string      `json:"label"`
}

// MappingTypes returns the domain mapping types in iota order.
func MappingTypes() []MappingTypeInfo {
	return []MappingTypeInfo{
		{Value: MappingTypeRename, Label: "RENAME"},
		{Value: MappingTypeSQL, Label: "SQL"},
		{Value: MappingTypePrefixPattern, Label: "PREFIX_PATTERN"},
		{Value: MappingTypeCustom, Label: "CUSTOM"},
		{Value: MappingTypeExtraFields, Label: "EXTRA_FIELDS"},
		{Value: MappingTypeMissingMappings, Label: "MISSING_MAPPINGS"},
		{Value: MappingTypeArray, Label: "ARRAY"},
	}
}

// Valid reports whether the integer is one of 0 through 6.
func (t MappingType) Valid() bool {
	return t.Label() != ""
}

// Label returns the domain name for a known mapping type.
func (t MappingType) Label() string {
	for _, info := range MappingTypes() {
		if info.Value == t {
			return info.Label
		}
	}

	return ""
}
