package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ECPartSuite struct {
	suite.Suite
}

func TestECPartSuite(t *testing.T) {
	suite.Run(t, new(ECPartSuite))
}

func (s *ECPartSuite) TestToMCP_MapsSubObjectAndPurchase() {
	p := repository.ECPart{
		ID: "p1", Brand: "DynaCap", Name: "Housings", PartType: repository.ECPartTypeHousings,
		Housings:   &repository.ECHousings{Size: new(repository.ECPartSize1U), Material: new("POM")},
		Purchases:  repository.BulkPurchases{{Vendor: new("Omnitype"), Price: new(10.0), Quantity: new(72), OrderDate: new("2026-09-21")}},
		Visibility: repository.VisibilityPrivate,
	}

	s.Equal(schema.ECPart{
		ID: "p1", Brand: "DynaCap", Name: "Housings", PartType: "housings",
		Housings:      &schema.ECHousings{Size: new("1U"), Material: new("POM")},
		Purchases:     []schema.ECPartPurchase{{Vendor: new("Omnitype"), Price: new(10.0), Currency: new("USD"), Quantity: new(72), OrderDate: new("2026-09-21")}},
		TotalQuantity: new(72),
		TotalCost:     new(10.0),
		Currency:      new("USD"),
		Visibility:    new("private"),
	}, ECPart{}.ToMCP(p, true, repository.ProfilePreferences{Currency: "USD"}))
}

func (s *ECPartSuite) TestToMCP_NonOwner_HidesVisibilityAndUnsharedPrice() {
	p := repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic, Purchases: repository.BulkPurchases{{Price: new(36.0)}}}

	out := ECPart{}.ToMCP(p, false, repository.ProfilePreferences{})

	s.Nil(out.Visibility)
	s.Require().Len(out.Purchases, 1)
	s.Nil(out.Purchases[0].Price)
}

func (s *ECPartSuite) TestToMCPSummary_LiftsOrderStatusAndFollowsSummaryPricePreference() {
	p := repository.ECPart{
		ID: "p1", Brand: "DynaCap", Name: "Heavy", PartType: repository.ECPartTypeDomes,
		Purchases:  repository.BulkPurchases{{Price: new(36.0), OrderStatus: new("Delivered")}},
		Visibility: repository.VisibilityPublic,
	}

	hidden := ECPart{}.ToMCPSummary(p, true, repository.ProfilePreferences{})
	shown := ECPart{}.ToMCPSummary(p, true, repository.ProfilePreferences{ShowPriceToMe: true, Currency: "USD"})

	s.Equal(schema.ECPartSummary{ID: "p1", Brand: "DynaCap", Name: "Heavy", PartType: "domes", OrderStatus: new("Delivered"), Visibility: new("public")}, hidden)
	s.Equal(new(36.0), shown.Price)
	s.Equal(new("USD"), shown.Currency)
}

func (s *ECPartSuite) TestFromMCP_MapsEveryField() {
	in := schema.ECPartInput{
		Brand: "DynaCap", Name: "Sliders", PartType: "sliders",
		Domes:      &schema.ECDomes{Weight: new(45.0)},
		Sliders:    &schema.ECSliders{Size: new("2U"), Stem: new("OEM"), Material: new("POM")},
		Housings:   &schema.ECHousings{Size: new("1U")},
		Springs:    &schema.ECSprings{Material: new("Nickel-Plated")},
		Rings:      &schema.ECRings{Thickness: new(0.5)},
		Purchases:  []schema.ECPartPurchaseInput{{Vendor: new("Omnitype"), DeliveryDate: new("2026-09-25")}},
		Notes:      new("n"),
		Visibility: "public",
	}

	s.Equal(repository.ECPart{
		Brand: "DynaCap", Name: "Sliders", PartType: repository.ECPartTypeSliders,
		Domes:      &repository.ECDomes{Weight: new(45.0)},
		Sliders:    &repository.ECSliders{Size: new(repository.ECPartSize2U), Stem: new(repository.ECSliderStemOEM), Material: new("POM")},
		Housings:   &repository.ECHousings{Size: new(repository.ECPartSize1U)},
		Springs:    &repository.ECSprings{Material: new("Nickel-Plated")},
		Rings:      &repository.ECRings{Thickness: new(0.5)},
		Purchases:  repository.BulkPurchases{{Vendor: new("Omnitype"), DeliveryDate: new("2026-09-25")}},
		Notes:      new("n"),
		Visibility: repository.VisibilityPublic,
	}, ECPart{}.FromMCP(in))
}

func (s *ECPartSuite) TestToMCP_Image_ReportsHasImageAndOwnerOnlyStatus() {
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/u1")
	p := repository.ECPart{ID: "p1", ImagePath: &key, ImageStatus: repository.ImageStatusPending}

	owner := ECPart{}.ToMCP(p, true, repository.ProfilePreferences{})
	other := ECPart{}.ToMCP(p, false, repository.ProfilePreferences{})

	s.True(owner.HasImage)
	s.Equal(new("pending"), owner.ImageStatus)
	s.False(other.HasImage)
	s.Nil(other.ImageStatus)
	s.False(ECPart{}.ToMCPSummary(p, false, repository.ProfilePreferences{}).HasImage)
	s.True(ECPart{}.ToMCPSummary(p, true, repository.ProfilePreferences{}).HasImage)
}

func (s *ECPartSuite) TestToMCP_DerivesOrderStatusAndTotalCostSkippingCancelled() {
	p := repository.ECPart{Purchases: repository.BulkPurchases{
		{Price: new(36.0), OrderStatus: new("Delivered")},
		{Price: new(28.8), OrderStatus: new("Ordered")},
		{Price: new(50.0), OrderStatus: new("Cancelled")},
	}}

	out := ECPart{}.ToMCP(p, true, repository.ProfilePreferences{Currency: "USD"})

	s.Equal(new("Ordered"), out.OrderStatus)
	s.Equal(new(64.8), out.TotalCost)
	s.Equal(new("USD"), out.Currency)
}

func (s *ECPartSuite) TestToMCP_NonOwner_HidesUnsharedTotalCostKeepsOrderStatus() {
	p := repository.ECPart{Purchases: repository.BulkPurchases{{Price: new(36.0), OrderStatus: new("Delivered")}}}

	out := ECPart{}.ToMCP(p, false, repository.ProfilePreferences{Currency: "USD"})

	s.Equal(new("Delivered"), out.OrderStatus)
	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}
