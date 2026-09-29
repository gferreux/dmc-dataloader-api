package model

// BigQuery type names used by templates and the mapping-type table.
const (
	BQString    = "STRING"
	BQInteger   = "INTEGER"
	BQFloat     = "FLOAT"
	BQBoolean   = "BOOLEAN"
	BQTimestamp = "TIMESTAMP"
	BQDate      = "DATE"
	BQJSON      = "JSON"
)

// MappingType is the integer stored on mappings.<column>.type.
//
// These values could not be read from github.com/dekuple-labs/dmc-domain in this
// environment. They are an explicit working table, not a dump of the loader enum:
// the seven integers cover the column kinds the import templates actually use.
// Confirm the order against dmodel before production writes.
const (
	MappingTypeString    MappingType = 0
	MappingTypeInteger   MappingType = 1
	MappingTypeFloat     MappingType = 2
	MappingTypeBoolean   MappingType = 3
	MappingTypeTimestamp MappingType = 4
	MappingTypeDate      MappingType = 5
	MappingTypeJSON      MappingType = 6
)

// MappingType is the Firestore integer discriminator for a mapped column.
type MappingType int

// MappingTypeInfo is one entry of GET /api/v1/meta mappingTypes.
type MappingTypeInfo struct {
	Value  MappingType `json:"value"`
	Label  string      `json:"label"`
	BQType string      `json:"bqType"`
}

// MappingTypes returns the working 0-6 table in order.
func MappingTypes() []MappingTypeInfo {
	return []MappingTypeInfo{
		{Value: MappingTypeString, Label: BQString, BQType: BQString},
		{Value: MappingTypeInteger, Label: BQInteger, BQType: BQInteger},
		{Value: MappingTypeFloat, Label: BQFloat, BQType: BQFloat},
		{Value: MappingTypeBoolean, Label: BQBoolean, BQType: BQBoolean},
		{Value: MappingTypeTimestamp, Label: BQTimestamp, BQType: BQTimestamp},
		{Value: MappingTypeDate, Label: BQDate, BQType: BQDate},
		{Value: MappingTypeJSON, Label: BQJSON, BQType: BQJSON},
	}
}

// Valid reports whether the integer is one of the known mapping types.
func (t MappingType) Valid() bool {
	return t.BQType() != ""
}

// BQType returns the BigQuery type name for a known mapping type.
func (t MappingType) BQType() string {
	for _, info := range MappingTypes() {
		if info.Value == t {
			return info.BQType
		}
	}

	return ""
}

// MappingTypeFromBQ maps a BigQuery type name onto the working integer table.
func MappingTypeFromBQ(bqType string) (MappingType, bool) {
	for _, info := range MappingTypes() {
		if info.BQType == bqType {
			return info.Value, true
		}
	}

	return 0, false
}

// MappingTypePtr returns a pointer to a copy of t.
func MappingTypePtr(t MappingType) *MappingType {
	copied := t

	return &copied
}
