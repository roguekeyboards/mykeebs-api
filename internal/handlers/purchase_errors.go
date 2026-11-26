package handlers

import (
	"net/http"

	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/purchasevalidate"
)

// writePurchaseErrors writes a 400 listing fieldErrs, if any.
func writePurchaseErrors(w http.ResponseWriter, fieldErrs []purchasevalidate.FieldError) (ok bool) {
	if len(fieldErrs) == 0 {
		return true
	}

	invalidParams := make([]problem.InvalidParam, len(fieldErrs))
	for i, fe := range fieldErrs {
		invalidParams[i] = problem.InvalidParam{Name: fe.Field, Reason: fe.Reason}
	}
	problem.ValidationFailed(w, "one or more fields are invalid", invalidParams)
	return false
}
