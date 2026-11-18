package lookup

// Category identifies a lookup category by name. It's a distinct type
// (not a bare string) so a typo in a category name is caught by the
// compiler wherever a Category is expected, rather than surfacing at
// runtime as an always-empty/always-invalid lookup.
type Category string

// Category names, matching internal/lookup/data/ exactly.
// Entity handlers reference these instead of re-typing category name
// strings, so a typo can't silently create a mismatch between a handler's
// checks and the seeded data.
const (
	// CategoryVisibility exists only so GET /v1/lookups[/{category}] can
	// list it like any other category - Visibility is a closed OpenAPI
	// enum/Go type (see [github.com/roguekeyboards/mykeebs-api/internal/repository.Visibility.Valid]),
	// never runtime-checked via validateFields the way the categories below are.
	CategoryVisibility                  Category = "visibility"
	CategoryKeyboardPCBAssemblyType     Category = "keyboard_pcb_assembly_type"
	CategoryKeyboardPCBConnectivityType Category = "keyboard_pcb_connectivity_type"
	CategoryKeyboardSize                Category = "keyboard_size"
	CategoryKeyboardLayout              Category = "keyboard_layout"
	CategoryKeyboardCaseMaterial        Category = "keyboard_case_material"
	CategoryKeyboardPlateMaterial       Category = "keyboard_plate_material"
	CategoryKeyboardWeightMaterial      Category = "keyboard_weight_material"
	CategoryKeyboardPCBFirmware         Category = "keyboard_pcb_firmware"
	CategorySwitchType                  Category = "switch_type"
	CategorySwitchMaterial              Category = "switch_material"
	CategorySwitchSpringMaterial        Category = "switch_spring_material"
	CategoryKeycapProfile               Category = "keycap_profile"
	CategoryKeycapMaterial              Category = "keycap_material"
	CategoryVendor                      Category = "vendor"
	CategoryOrderStatus                 Category = "order_status"
	CategoryBuildStabilizer             Category = "build_stabilizer"
	CategoryBuildStabilizerMountType    Category = "build_stabilizer_mount_type"
	CategoryBuildCaseMountType          Category = "build_case_mount_type"
	CategoryBuildDurometer              Category = "build_durometer"
	CategoryBuildType                   Category = "build_type"
	CategoryECDomeMaterial              Category = "ec_dome_material"
	CategoryECRingMaterial              Category = "ec_ring_material"
	CategoryImageContentType            Category = "image_content_type"
)
