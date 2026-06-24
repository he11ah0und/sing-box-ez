package singboxconfig

// Schema is the root of the sing-box field dictionary.
type Schema struct {
	Version       string                 `yaml:"version"`
	SingboxLatest string                 `yaml:"singbox_latest"`
	Fields        map[string]*SchemaNode `yaml:"fields"`
}

// SchemaNode describes a single field in the sing-box config.
type SchemaNode struct {
	Since       string                 `yaml:"since"`
	SinceV      Version                `yaml:"-"`
	Deprecated  string                 `yaml:"deprecated,omitempty"`
	DeprecatedV Version                `yaml:"-"`
	Removed     string                 `yaml:"removed,omitempty"`
	RemovedV    Version                `yaml:"-"`
	Replacement string                 `yaml:"replacement,omitempty"`
	RenameTo    string                 `yaml:"rename_to,omitempty"`
	Type        string                 `yaml:"type"`
	Children    map[string]*SchemaNode `yaml:"children,omitempty"`
	Items       *SchemaNode            `yaml:"items,omitempty"`
	OneOfBy     string                 `yaml:"one_of_by,omitempty"`
	OneOf       []TypedVariant         `yaml:"one_of,omitempty"`
	// LegacyHint marks fields that need special semantic checks beyond the dictionary.
	LegacyHint string `yaml:"legacy_hint,omitempty"`
}

// TypedVariant selects a set of fields when a discriminator matches.
type TypedVariant struct {
	When   map[string]string      `yaml:"when"`
	Fields map[string]*SchemaNode `yaml:"fields"`
}

// FieldInfo is a flattened view of a schema node, including its full JSON path.
type FieldInfo struct {
	Path        string
	Since       Version
	Deprecated  Version
	Removed     Version
	Replacement string
	RenameTo    string
	Type        string
	LegacyHint  string
}
