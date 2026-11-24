package repository_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type BulkPurchasesSuite struct {
	suite.Suite
}

func TestBulkPurchasesSuite(t *testing.T) {
	suite.Run(t, new(BulkPurchasesSuite))
}

func (s *BulkPurchasesSuite) TestTotalQuantity_SumsKnownQuantities() {
	ps := repository.BulkPurchases{{Quantity: new(70)}, {}, {Quantity: new(35)}}

	s.Equal(new(105), ps.TotalQuantity())
}

func (s *BulkPurchasesSuite) TestTotalQuantity_NoneKnown_ReturnsNil() {
	s.Nil(repository.BulkPurchases{{Price: new(10.0)}}.TotalQuantity())
	s.Nil(repository.BulkPurchases(nil).TotalQuantity())
}

func (s *BulkPurchasesSuite) TestTotalPrice_SumsKnownPricesRoundedToCents() {
	ps := repository.BulkPurchases{{Price: new(0.1)}, {Price: new(0.2)}, {}}

	s.Equal(new(0.3), ps.TotalPrice())
}

func (s *BulkPurchasesSuite) TestTotalPrice_NoneKnown_ReturnsNil() {
	s.Nil(repository.BulkPurchases{{Quantity: new(1)}}.TotalPrice())
}

func (s *BulkPurchasesSuite) TestUnitPrice_AveragesAcrossPurchasesWithPriceAndQuantity() {
	ps := repository.BulkPurchases{
		{Price: new(36.0), Quantity: new(90)},
		{Price: new(54.0), Quantity: new(90)},
		{Price: new(100.0)},
		{Quantity: new(500)},
	}

	got := ps.UnitPrice()

	s.Require().NotNil(got)
	s.InDelta(0.5, *got, 1e-9)
}

func (s *BulkPurchasesSuite) TestUnitPrice_NoneHasBoth_ReturnsNil() {
	ps := repository.BulkPurchases{{Price: new(36.0)}, {Quantity: new(90)}, {Price: new(1.0), Quantity: new(0)}}

	s.Nil(ps.UnitPrice())
}

func (s *BulkPurchasesSuite) TestOrderStatus_LeastProgressedWins() {
	ps := repository.BulkPurchases{{OrderStatus: new("Delivered")}, {OrderStatus: new("Ordered")}, {OrderStatus: new("Cancelled")}}

	s.Equal(new("Ordered"), ps.OrderStatus())
}

func (s *BulkPurchasesSuite) TestOrderStatus_NoneSet_ReturnsNil() {
	s.Nil(repository.BulkPurchases{{}}.OrderStatus())
}

func (s *BulkPurchasesSuite) TestTotals_SkipCancelledPurchases() {
	ps := repository.BulkPurchases{
		{Price: new(35.0), Quantity: new(70), OrderStatus: new("Delivered")},
		{Price: new(50.0), Quantity: new(70), OrderStatus: new("Cancelled")},
	}

	s.Equal(new(70), ps.TotalQuantity())
	s.Equal(new(35.0), ps.TotalPrice())
	s.Require().NotNil(ps.UnitPrice())
	s.InDelta(0.5, *ps.UnitPrice(), 1e-9)
}

func (s *BulkPurchasesSuite) TestTotals_AllCancelled_ReturnNil() {
	ps := repository.BulkPurchases{{Price: new(50.0), Quantity: new(70), OrderStatus: new("Cancelled")}}

	s.Nil(ps.TotalQuantity())
	s.Nil(ps.TotalPrice())
	s.Nil(ps.UnitPrice())
}

func (s *BulkPurchasesSuite) TestWithoutEmpty_DropsPurchasesWithNoFieldSet() {
	ps := repository.BulkPurchases{{}, {Vendor: new("Omnitype")}, {}}

	s.Equal(repository.BulkPurchases{{Vendor: new("Omnitype")}}, ps.WithoutEmpty())
}

func (s *BulkPurchasesSuite) TestWithoutEmpty_AllEmpty_ReturnsNil() {
	s.Nil(repository.BulkPurchases{{}, {}}.WithoutEmpty())
}
