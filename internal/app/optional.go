package app

// OptionalString models a CLI flag that may be unset, set to a value, or
// explicitly cleared (empty string after Flags().Changed).
// Unset: Set == false. Set (including clear): Set == true, Value may be "".
type OptionalString struct {
	Set   bool
	Value string
}
