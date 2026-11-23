// Package buildrefs validates that a Build's cross-entity references
// (Keyboard, Switches, ECParts, KeycapKits) name real resources owned by
// the caller, that its Plate and PCB are ones its keyboard has, and that it
// lists the part kind its build type takes. Separate from
// [github.com/roguekeyboards/mykeebs-api/internal/lookup] since it checks a
// build's references, not the static lookup category system.
package buildrefs
