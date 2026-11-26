// Package purchasevalidate holds the purchase rule api/openapi.yaml can't
// express: delivery_date and estimated_delivery_date can't be before
// order_date. Shared by the REST handlers and the MCP tools so the rule
// can't drift.
package purchasevalidate

import (
	"fmt"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// FieldError reports that Field failed for Reason.
type FieldError struct {
	Field  string
	Reason string
}

// KeyboardPurchase validates the purchase of a keyboard, plate or PCB.
func KeyboardPurchase(p repository.KeyboardPurchase) []FieldError {
	return dates("purchase", p.OrderDate, p.DeliveryDate, p.EstimatedDeliveryDate)
}

// KeycapKitPurchase validates a keycap kit's purchase.
func KeycapKitPurchase(p repository.KeycapKitPurchase) []FieldError {
	return dates("purchase", p.OrderDate, p.DeliveryDate, p.EstimatedDeliveryDate)
}

// BulkPurchases validates every purchase of a switch or EC part.
func BulkPurchases(ps repository.BulkPurchases) []FieldError {
	var errs []FieldError
	for i, p := range ps {
		errs = append(errs, dates(fmt.Sprintf("purchases[%d]", i), p.OrderDate, p.DeliveryDate, p.EstimatedDeliveryDate)...)
	}
	return errs
}

// dates compares the dates as strings, which orders correctly because both
// transports have already checked they're YYYY-MM-DD.
func dates(prefix string, orderDate, deliveryDate, estimatedDeliveryDate *string) []FieldError {
	if orderDate == nil {
		return nil
	}

	var errs []FieldError
	for _, d := range []struct {
		field string
		value *string
	}{
		{"delivery_date", deliveryDate},
		{"estimated_delivery_date", estimatedDeliveryDate},
	} {
		if d.value != nil && *d.value < *orderDate {
			errs = append(errs, FieldError{
				Field:  prefix + "." + d.field,
				Reason: "can't be before order_date",
			})
		}
	}
	return errs
}
