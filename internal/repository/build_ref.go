package repository

// RefType identifies which of a Build's reference fields a refMarker (see
// [github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo]) points at.
// Used internally to build marker items and reverse-lookup queries - never
// appears in BuildRepository's exported FindBuildsReferencingX method
// signatures, which are typed per reference kind instead.
type RefType string

const (
	// RefTypeKeyboard marks a marker for a Build's Keyboard field.
	RefTypeKeyboard RefType = "keyboard"
	// RefTypeSwitch marks a marker for one of a Build's Switches[] entries.
	RefTypeSwitch RefType = "switch"
	// RefTypeECPart marks a marker for one of a Build's ECParts[] entries.
	RefTypeECPart RefType = "ec_part"
	// RefTypeKeycapKit marks a marker for one of a Build's KeycapKits[]
	// entries.
	RefTypeKeycapKit RefType = "keycap_kit"
)
