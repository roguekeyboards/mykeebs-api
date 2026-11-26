package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type KeyboardToMCPSuite struct {
	suite.Suite
}

func TestKeyboardToMCPSuite(t *testing.T) {
	suite.Run(t, new(KeyboardToMCPSuite))
}

func (s *KeyboardToMCPSuite) TestMapsAllFields() {
	size := "65%"
	layout := "ANSI"
	material := "Aluminum"
	color := "Silver"
	thickness := 1.6
	firmware := "QMK/VIA"
	vendor := "Divinikey"
	status := "Delivered"
	notes := "daily driver"

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:     "kb-1",
		Brand:  "Mode",
		Name:   "Sixty",
		Size:   &size,
		Layout: &layout,
		Design: repository.KeyboardDesign{
			TopCase: repository.KeyboardMaterialColor{Material: &material, Color: &color},
		},
		Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{
			{ID: "p1", Material: "Brass"},
			{ID: "p2", Material: "POM", Color: &color, Purchase: repository.KeyboardPurchase{Vendor: &vendor, Price: new(25.0)}},
		}),
		PCBs:       repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b1", Thickness: &thickness, Firmware: &firmware}}),
		Purchase:   repository.KeyboardPurchase{Vendor: &vendor, OrderStatus: &status, Price: new(300.0)},
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{Currency: "USD"})

	s.Equal("kb-1", out.ID)
	s.Require().NotNil(out.Size)
	s.Equal("65%", *out.Size)
	s.Require().NotNil(out.Design)
	s.Require().NotNil(out.Design.TopCase)
	s.Equal("Aluminum", *out.Design.TopCase.Material)
	s.Equal([]schema.KeyboardPlate{
		{ID: "p1", Material: "Brass"},
		{ID: "p2", Material: "POM", Color: &color, Purchase: &schema.KeyboardPartPurchase{Vendor: &vendor, Price: new(25.0), Currency: new("USD")}},
	}, out.Plates)
	s.Require().Len(out.PCBs, 1)
	s.Equal("b1", out.PCBs[0].ID)
	s.InDelta(1.6, *out.PCBs[0].Thickness, 0.001)
	s.Nil(out.PCBs[0].Purchase)
	s.Equal(new(325.0), out.TotalCost)
	s.Require().NotNil(out.Purchase)
	s.Equal("Delivered", *out.Purchase.OrderStatus)
	s.Require().NotNil(out.Visibility)
	s.Equal("public", *out.Visibility)
}

func (s *KeyboardToMCPSuite) TestNonOwner_OmitsVisibility() {
	out := Keyboard{}.ToMCP(repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPublic}, false, repository.ProfilePreferences{})

	s.Nil(out.Visibility)
}

func (s *KeyboardToMCPSuite) TestEmptyGroups_CollapseToNil() {
	out := Keyboard{}.ToMCP(repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, true, repository.ProfilePreferences{})

	s.Nil(out.Design)
	s.Nil(out.Plates)
	s.Nil(out.PCBs)
	s.Nil(out.Purchase)
	s.Nil(out.TotalCost)
	s.Nil(out.Size)
	s.Nil(out.Layout)
	s.Nil(out.Notes)
}

func (s *KeyboardToMCPSuite) TestDesignWithOnlyOnePart_IsRetained() {
	material := "PC"

	out := Keyboard{}.ToMCP(repository.Keyboard{
		Design: repository.KeyboardDesign{
			Weight: repository.KeyboardMaterialColor{Material: &material},
		},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Design)
	s.Require().NotNil(out.Design.Weight)
	s.Equal("PC", *out.Design.Weight.Material)
	s.Nil(out.Design.TopCase)
	s.Nil(out.Design.BottomCase)
}

func (s *KeyboardToMCPSuite) TestOnlyEstimatedDeliveryDate_KeepsPurchase() {
	out := Keyboard{}.ToMCP(repository.Keyboard{
		Purchase: repository.KeyboardPurchase{EstimatedDeliveryDate: new("2026-12-31")},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Purchase)
	s.Equal(new("2026-12-31"), out.Purchase.EstimatedDeliveryDate)
}

// A recorded zero must stay distinct from an unset field, or MCP would
// report "not recorded" for a keyboard REST reports as 0.
func (s *KeyboardToMCPSuite) TestRecordedZero_SurvivesRoundTrip() {
	price := 0.0
	thickness := 0.0

	out := Keyboard{}.ToMCP(repository.Keyboard{
		PCBs:     repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b1", Thickness: &thickness}}),
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{})

	s.Require().Len(out.PCBs, 1)
	s.Require().NotNil(out.PCBs[0].Thickness)
	s.Zero(*out.PCBs[0].Thickness)
	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.Zero(*out.Purchase.Price)
}

func (s *KeyboardToMCPSuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Divinikey"
	status := "Delivered"
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Vendor: &vendor, OrderStatus: &status, Price: &price},
		PCBs:       repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b1", Purchase: repository.KeyboardPurchase{Price: &price}}}),
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Require().Len(out.PCBs, 1)
	s.Require().NotNil(out.PCBs[0].Purchase)
	s.Nil(out.PCBs[0].Purchase.Price)
	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(&vendor, out.Purchase.Vendor)
	s.Equal(&status, out.Purchase.OrderStatus)
	s.Nil(out.Purchase.Currency)
}

func (s *KeyboardToMCPSuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Price: &price},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

func (s *KeyboardToMCPSuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Price: &price},
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

type KeyboardToMCPSummarySuite struct {
	suite.Suite
}

func TestKeyboardToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(KeyboardToMCPSummarySuite))
}

// The summary reaches into purchase for order_status, unlike switches'
// flat summary - a keyboard on order is the case worth surfacing in a list.
func (s *KeyboardToMCPSummarySuite) TestIncludesOrderStatusFromPurchase() {
	size := "TKL"
	status := "Shipped"

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Brand:    "Mode",
		Name:     "Sixty",
		Size:     &size,
		Purchase: repository.KeyboardPurchase{OrderStatus: &status},
	}, true, repository.ProfilePreferences{})

	s.Equal("kb-1", out.ID)
	s.Equal("TKL", *out.Size)
	s.Require().NotNil(out.OrderStatus)
	s.Equal("Shipped", *out.OrderStatus)
}

func (s *KeyboardToMCPSummarySuite) TestNoPurchase_LeavesOrderStatusNil() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{ID: "kb-1"}, true, repository.ProfilePreferences{})

	s.Nil(out.OrderStatus)
}

func (s *KeyboardToMCPSummarySuite) TestOwnerShowPriceToMeTrue_IncludesTotalCost() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true})

	s.Require().NotNil(out.TotalCost)
	s.InDelta(price, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *KeyboardToMCPSummarySuite) TestTotalCost_IncludesPartPrices() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: new(300.0)},
		Plates:   repository.KeyboardPlatesMap([]repository.KeyboardPlate{{ID: "p1", Material: "AL", Purchase: repository.KeyboardPurchase{Price: new(40.0)}}}),
	}, true, repository.ProfilePreferences{ShowPriceToMe: true})

	s.Equal(new(340.0), out.TotalCost)
}

func (s *KeyboardToMCPSummarySuite) TestOwnerShowPriceToMeFalse_OmitsTotalCost() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwnerShowPriceToOthersFalse_OmitsTotalCost() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwnerShowPriceToOthersTrue_IncludesTotalCost() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().NotNil(out.TotalCost)
	s.InDelta(price, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

type KeyboardFromMCPSuite struct {
	suite.Suite
}

func TestKeyboardFromMCPSuite(t *testing.T) {
	suite.Run(t, new(KeyboardFromMCPSuite))
}

func (s *KeyboardFromMCPSuite) TestMapsAllFields() {
	size := "60%"
	material := "Aluminum"
	price := 0.0

	out := Keyboard{}.FromMCP(schema.KeyboardInput{
		Brand: "Mode",
		Name:  "Sixty",
		Size:  &size,
		Design: &schema.KeyboardDesign{
			TopCase: &schema.KeyboardMaterialColor{Material: &material},
		},
		Purchase:   &schema.KeyboardPurchaseInput{Price: &price},
		Visibility: "public",
	})

	s.Equal("Mode", out.Brand)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Equal("60%", *out.Size)
	s.Equal("Aluminum", *out.Design.TopCase.Material)
	s.Zero(*out.Purchase.Price, "a recorded zero price must survive the inbound mapping too")
}

func (s *KeyboardFromMCPSuite) TestPlateFromMCP_MapsAllFields() {
	out := Keyboard{}.PlateFromMCP(schema.KeyboardPlateInput{
		Material: "Brass", Color: new("Raw"), Thickness: new(1.5),
		Purchase: &schema.KeyboardPartPurchaseInput{Vendor: new("Divinikey"), OrderDate: new("2026-02-01"), EstimatedDeliveryDate: new("2026-06-30")},
	})

	s.Equal(repository.KeyboardPlate{
		Material: "Brass", Color: new("Raw"), Thickness: new(1.5),
		Purchase: repository.KeyboardPurchase{Vendor: new("Divinikey"), OrderDate: new("2026-02-01"), EstimatedDeliveryDate: new("2026-06-30")},
	}, out)
}

func (s *KeyboardFromMCPSuite) TestPCBFromMCP_MapsAllFields() {
	firmware := "QMK/VIA"

	out := Keyboard{}.PCBFromMCP(schema.KeyboardPCBInput{
		Firmware: &firmware, Assembly: new("EC"), Connectivity: new("Wired"), Thickness: new(1.6),
	})

	s.Equal(repository.KeyboardPCB{Firmware: &firmware, Assembly: new("EC"), Connectivity: new("Wired"), Thickness: new(1.6)}, out)
}

// ID and UserID are set by the caller and the repository layer
// respectively, never by the tool argument.
func (s *KeyboardFromMCPSuite) TestLeavesIdentityUnset() {
	out := Keyboard{}.FromMCP(schema.KeyboardInput{Brand: "B", Name: "N", Visibility: "private"})

	s.Empty(out.ID)
	s.Empty(out.UserID)
}

func (s *KeyboardFromMCPSuite) TestNilGroups_MapToZeroValues() {
	out := Keyboard{}.FromMCP(schema.KeyboardInput{Brand: "B", Name: "N", Visibility: "private"})

	s.Nil(out.Design.TopCase.Material)
	s.Nil(out.Plates)
	s.Nil(out.PCBs)
	s.Nil(out.Purchase.Vendor)
}

func (s *KeyboardToMCPSummarySuite) TestOwner_IncludesVisibility() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:         "kb-1",
		Visibility: repository.VisibilityPrivate,
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Visibility)
	s.Equal("private", *out.Visibility)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwner_OmitsVisibility() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:         "kb-1",
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Nil(out.Visibility)
}
