package mcp

import (
	"errors"
	"strings"

	"github.com/roguekeyboards/mykeebs-api/internal/purchasevalidate"
)

// purchaseErrors joins fieldErrs into one tool error, or returns nil.
func purchaseErrors(fieldErrs []purchasevalidate.FieldError) error {
	if len(fieldErrs) == 0 {
		return nil
	}

	reasons := make([]string, len(fieldErrs))
	for i, fe := range fieldErrs {
		reasons[i] = fe.Field + ": " + fe.Reason
	}
	return errors.New(strings.Join(reasons, "; "))
}
