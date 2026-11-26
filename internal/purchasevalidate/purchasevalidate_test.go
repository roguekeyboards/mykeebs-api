package purchasevalidate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/purchasevalidate"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ValidateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(ValidateSuite))
}

func ptr(s string) *string { return &s }

func (s *ValidateSuite) TestNoOrderDate_NoErrors() {
	s.Empty(purchasevalidate.KeyboardPurchase(repository.KeyboardPurchase{
		DeliveryDate:          ptr("2026-01-01"),
		EstimatedDeliveryDate: ptr("2026-01-01"),
	}))
}

func (s *ValidateSuite) TestDatesOnOrAfterOrderDate_NoErrors() {
	s.Empty(purchasevalidate.KeyboardPurchase(repository.KeyboardPurchase{
		OrderDate:             ptr("2026-03-01"),
		DeliveryDate:          ptr("2026-03-01"),
		EstimatedDeliveryDate: ptr("2026-06-30"),
	}))
}

func (s *ValidateSuite) TestDatesBeforeOrderDate_EachFlagged() {
	s.Equal([]purchasevalidate.FieldError{
		{Field: "purchase.delivery_date", Reason: "can't be before order_date"},
		{Field: "purchase.estimated_delivery_date", Reason: "can't be before order_date"},
	}, purchasevalidate.KeyboardPurchase(repository.KeyboardPurchase{
		OrderDate:             ptr("2026-03-01"),
		DeliveryDate:          ptr("2026-02-28"),
		EstimatedDeliveryDate: ptr("2025-12-31"),
	}))
}

func (s *ValidateSuite) TestEstimateBeforeDelivery_NoErrors() {
	s.Empty(purchasevalidate.KeycapKitPurchase(repository.KeycapKitPurchase{
		OrderDate:             ptr("2026-03-01"),
		DeliveryDate:          ptr("2026-09-01"),
		EstimatedDeliveryDate: ptr("2026-06-30"),
	}))
}

func (s *ValidateSuite) TestKeycapKitEstimateBeforeOrderDate_Flagged() {
	s.Equal([]purchasevalidate.FieldError{
		{Field: "purchase.estimated_delivery_date", Reason: "can't be before order_date"},
	}, purchasevalidate.KeycapKitPurchase(repository.KeycapKitPurchase{
		OrderDate:             ptr("2026-03-01"),
		EstimatedDeliveryDate: ptr("2026-02-01"),
	}))
}

func (s *ValidateSuite) TestBulkPurchases_FlaggedByIndex() {
	s.Equal([]purchasevalidate.FieldError{
		{Field: "purchases[1].delivery_date", Reason: "can't be before order_date"},
	}, purchasevalidate.BulkPurchases(repository.BulkPurchases{
		{OrderDate: ptr("2026-03-01"), DeliveryDate: ptr("2026-03-05")},
		{OrderDate: ptr("2026-04-01"), DeliveryDate: ptr("2026-03-31")},
	}))
}
