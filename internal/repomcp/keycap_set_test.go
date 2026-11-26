package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type KeycapSetToMCPSuite struct {
	suite.Suite
}

func TestKeycapSetToMCPSuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToMCPSuite))
}

func (s *KeycapSetToMCPSuite) TestMapsAllFields() {
	profile := "Cherry"
	material := "ABS"
	notes := "grail set"
	imagePath := repository.KeycapKitImageKey("keycap-sets/u-1/ks-1/kits/kit-1/image/up1")
	vendor := "Amazon"

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Brand:      "GMK",
		Name:       "Olivia",
		Profile:    &profile,
		Material:   &material,
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
		Kits: map[string]repository.KeycapKit{
			"kit-1": {
				KitID:     "kit-1",
				Name:      "Base",
				ImagePath: &imagePath,
				Purchase:  repository.KeycapKitPurchase{Vendor: &vendor},
			},
			"kit-2": {KitID: "kit-2", Name: "Novelties"},
		},
	}, true, repository.ProfilePreferences{})

	s.Equal("ks-1", out.ID)
	s.Equal("GMK", out.Brand)
	s.Require().NotNil(out.Profile)
	s.Equal("Cherry", *out.Profile)
	s.Require().NotNil(out.Visibility)
	s.Equal("public", *out.Visibility)
	s.Require().Len(out.Kits, 2)

	s.Equal("kit-1", out.Kits[0].KitID)
	s.True(out.Kits[0].HasImage)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Equal("Amazon", *out.Kits[0].Purchase.Vendor)

	s.Equal("kit-2", out.Kits[1].KitID)
	s.False(out.Kits[1].HasImage)
	s.Nil(out.Kits[1].Purchase)
}

func (s *KeycapSetToMCPSuite) TestNonOwner_OmitsVisibility() {
	out := KeycapSet{}.ToMCP(repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPublic}, false, repository.ProfilePreferences{})

	s.Nil(out.Visibility)
}

func (s *KeycapSetToMCPSuite) TestPrimaryKitID_StillExists_IsPreserved() {
	kitID := "kit-1"

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:           "ks-1",
		Visibility:   repository.VisibilityPrivate,
		PrimaryKitID: &kitID,
		Kits:         map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Name: "Base"}},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.PrimaryKitID)
	s.Equal("kit-1", *out.PrimaryKitID)
}

func (s *KeycapSetToMCPSuite) TestPrimaryKitID_KitDeleted_IsNil() {
	dangling := "no-longer-a-kit"

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:           "ks-1",
		Visibility:   repository.VisibilityPrivate,
		PrimaryKitID: &dangling,
		Kits:         map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Name: "Base"}},
	}, true, repository.ProfilePreferences{})

	s.Nil(out.PrimaryKitID)
}

func (s *KeycapSetToMCPSuite) TestKits_ListedInSeqOrder() {
	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPrivate,
		Kits: map[string]repository.KeycapKit{
			"aaa": {KitID: "aaa", Name: "Added last", Seq: 20},
			"zzz": {KitID: "zzz", Name: "Added first", Seq: 10},
		},
	}, true, repository.ProfilePreferences{})

	s.Require().Len(out.Kits, 2)
	s.Equal("zzz", out.Kits[0].KitID)
	s.Equal("aaa", out.Kits[1].KitID)
}

func (s *KeycapSetToMCPSuite) TestOrderStatus_SetsAggregateFromKits() {
	ordered := "Ordered"
	shipped := "Shipped"

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPrivate,
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{OrderStatus: &shipped}},
			"kit-2": {KitID: "kit-2", Purchase: repository.KeycapKitPurchase{OrderStatus: &ordered}},
		},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.OrderStatus)
	s.Equal("Ordered", *out.OrderStatus)
}

func (s *KeycapSetToMCPSuite) TestNilKits_MapsToNilSlice() {
	out := KeycapSet{}.ToMCP(repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, true, repository.ProfilePreferences{})

	s.Nil(out.Kits)
}

func (s *KeycapSetToMCPSuite) TestNonOwnerShowPriceToOthersFalse_OmitsKitPriceKeepsRestOfPurchase() {
	vendor := "Amazon"
	price := 120.0

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Vendor: &vendor, Price: &price}},
		},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Require().Len(out.Kits, 1)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Nil(out.Kits[0].Purchase.Price)
	s.Equal(&vendor, out.Kits[0].Purchase.Vendor)
}

func (s *KeycapSetToMCPSuite) TestNonOwnerShowPriceToOthersTrue_IncludesKitPrice() {
	price := 120.0

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}},
		},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Require().Len(out.Kits, 1)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Require().NotNil(out.Kits[0].Purchase.Price)
	s.InDelta(price, *out.Kits[0].Purchase.Price, 0.0001)
}

func (s *KeycapSetToMCPSuite) TestOwner_AlwaysIncludesKitPriceRegardlessOfShowPriceToMe() {
	price := 120.0

	out := KeycapSet{}.ToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}},
		},
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Require().Len(out.Kits, 1)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Require().NotNil(out.Kits[0].Purchase.Price)
	s.InDelta(price, *out.Kits[0].Purchase.Price, 0.0001)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary() {
	profile := "OEM"
	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:      "ks-1",
		Brand:   "GMK",
		Name:    "Olivia",
		Profile: &profile,
	}, true, repository.ProfilePreferences{})

	s.Equal("ks-1", out.ID)
	s.Equal("GMK", out.Brand)
	s.Require().NotNil(out.Profile)
	s.Equal("OEM", *out.Profile)
	s.Nil(out.PrimaryKitID)
	s.False(out.PrimaryKitHasImage)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_OrderStatus_SetsAggregateFromKits() {
	delivered := "Delivered"
	cancelled := "Cancelled"

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID: "ks-1",
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{OrderStatus: &delivered}},
			"kit-2": {KitID: "kit-2", Purchase: repository.KeycapKitPurchase{OrderStatus: &cancelled}},
		},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.OrderStatus)
	s.Equal("Delivered", *out.OrderStatus)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_PrimaryKitWithImage_ReportsHasImageTrue() {
	imagePath := repository.KeycapKitImageKey("keycap-sets/u-1/ks-1/kits/kit-1/image/up1")

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:           "ks-1",
		PrimaryKitID: strPtr("kit-1"),
		Kits:         map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &imagePath}},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.PrimaryKitID)
	s.Equal("kit-1", *out.PrimaryKitID)
	s.True(out.PrimaryKitHasImage)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_PrimaryKitDeleted_NilAndFalse() {
	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:           "ks-1",
		PrimaryKitID: strPtr("no-longer-a-kit"),
		Kits:         map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}},
	}, true, repository.ProfilePreferences{})

	s.Nil(out.PrimaryKitID)
	s.False(out.PrimaryKitHasImage)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_OwnerShowPriceToMeTrue_SumsKnownKitPrices() {
	price1 := 120.00
	price2 := 35.00

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID: "ks-1",
		Kits: map[string]repository.KeycapKit{
			"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price1}},
			"kit-2": {KitID: "kit-2", Purchase: repository.KeycapKitPurchase{Price: &price2}},
		},
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true})

	s.Require().NotNil(out.TotalCost)
	s.InDelta(155.00, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_NoPricedKits_NilTotalCost() {
	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:   "ks-1",
		Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}},
	}, true, repository.ProfilePreferences{ShowPriceToMe: true})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_OwnerShowPriceToMeFalse_OmitsTotalCost() {
	price := 120.00

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:   "ks-1",
		Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_NonOwnerShowPriceToOthersFalse_OmitsTotalCost() {
	price := 120.00

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:   "ks-1",
		Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary_NonOwnerShowPriceToOthersTrue_IncludesTotalCost() {
	price := 120.00

	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:   "ks-1",
		Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().NotNil(out.TotalCost)
	s.InDelta(price, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_NoPurchaseFields_OmitsPurchase() {
	out := KeycapSet{}.KitToMCP(repository.KeycapKit{KitID: "kit-1", Name: "Base"}, true, repository.ProfilePreferences{Currency: "EUR"})

	s.Equal("kit-1", out.KitID)
	s.False(out.HasImage)
	s.Nil(out.Purchase)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_OnlyEstimatedDeliveryDate_KeepsPurchase() {
	out := KeycapSet{}.KitToMCP(repository.KeycapKit{
		KitID:    "kit-1",
		Name:     "Base",
		Purchase: repository.KeycapKitPurchase{EstimatedDeliveryDate: new("2026-12-31")},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Purchase)
	s.Equal(new("2026-12-31"), out.Purchase.EstimatedDeliveryDate)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_NonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Amazon"
	price := 120.0

	out := KeycapSet{}.KitToMCP(repository.KeycapKit{
		KitID:    "kit-1",
		Name:     "Base",
		Purchase: repository.KeycapKitPurchase{Vendor: &vendor, Price: &price},
	}, false, repository.ProfilePreferences{Currency: "EUR"})

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Nil(out.Purchase.Currency)
	s.Equal(&vendor, out.Purchase.Vendor)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_Owner_IncludesPrice() {
	price := 120.0

	out := KeycapSet{}.KitToMCP(repository.KeycapKit{
		KitID:    "kit-1",
		Name:     "Base",
		Purchase: repository.KeycapKitPurchase{Price: &price},
	}, true, repository.ProfilePreferences{Currency: "EUR"})

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetFromMCP_MapsAllFields() {
	profile := "Cherry"
	material := "ABS"
	notes := "grail set"

	out := KeycapSet{}.FromMCP(schema.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Olivia",
		Profile:    &profile,
		Material:   &material,
		Notes:      &notes,
		Visibility: "public",
	})

	s.Equal("GMK", out.Brand)
	s.Equal("Olivia", out.Name)
	s.Require().NotNil(out.Profile)
	s.Equal("Cherry", *out.Profile)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Empty(out.ID, "ID is the caller's responsibility, not this mapping's")
	s.Nil(out.Kits, "a set write never carries kits")
}

func (s *KeycapSetToMCPSuite) TestKeycapKitFromMCP_MapsAllFields() {
	vendor := "Amazon"
	price := 120.0
	orderDate := "2026-01-15"

	out := KeycapSet{}.KitFromMCP(schema.KeycapKitInput{
		Name: "Base",
		Purchase: &schema.KeycapKitPurchaseInput{
			Vendor:                &vendor,
			Price:                 &price,
			OrderDate:             &orderDate,
			EstimatedDeliveryDate: new("2026-06-30"),
		},
	})

	s.Equal("Base", out.Name)
	s.Equal(new("2026-06-30"), out.Purchase.EstimatedDeliveryDate)
	s.Require().NotNil(out.Purchase.Vendor)
	s.Equal("Amazon", *out.Purchase.Vendor)
	s.Require().NotNil(out.Purchase.Price)
	s.InEpsilon(120.0, *out.Purchase.Price, 0.001)
	s.Empty(out.KitID, "KitID is the caller's responsibility, not this mapping's")
	s.Nil(out.ImagePath, "a kit write never carries its image path")
}

func (s *KeycapSetToMCPSuite) TestKeycapKitFromMCP_NoPurchase_MapsToZeroValue() {
	out := KeycapSet{}.KitFromMCP(schema.KeycapKitInput{Name: "Base"})

	s.Equal(repository.KeycapKitPurchase{}, out.Purchase)
}

func (s *KeycapSetToMCPSuite) TestToMCPSummary_Owner_IncludesSetsOwnVisibility() {
	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPrivate,
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Visibility)
	s.Equal("private", *out.Visibility)
}

func (s *KeycapSetToMCPSuite) TestToMCPSummary_NonOwner_OmitsVisibility() {
	out := KeycapSet{}.ToMCPSummary(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Nil(out.Visibility)
}

func pricedMCPKeycapSet() repository.KeycapSet {
	return repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{
		"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: new(120.0)}},
		"kit-2": {KitID: "kit-2", Purchase: repository.KeycapKitPurchase{Price: new(35.0)}},
	}}
}

func (s *KeycapSetToMCPSuite) TestToMCP_Owner_IncludesTotalCostRegardlessOfShowPriceToMe() {
	out := KeycapSet{}.ToMCP(pricedMCPKeycapSet(), true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})

	s.Equal(new(155.0), out.TotalCost)
	s.Equal(new("EUR"), out.Currency)
}

func (s *KeycapSetToMCPSuite) TestToMCP_NonOwnerShowPriceToOthersTrue_IncludesTotalCost() {
	out := KeycapSet{}.ToMCP(pricedMCPKeycapSet(), false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Equal(new(155.0), out.TotalCost)
	s.Equal(new("EUR"), out.Currency)
}

func (s *KeycapSetToMCPSuite) TestToMCP_NonOwnerShowPriceToOthersFalse_OmitsTotalCost() {
	out := KeycapSet{}.ToMCP(pricedMCPKeycapSet(), false, repository.ProfilePreferences{Currency: "EUR"})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}
