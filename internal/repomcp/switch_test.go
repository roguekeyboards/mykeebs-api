package repomcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type SwitchToMCPSuite struct {
	suite.Suite
}

func TestSwitchToMCPSuite(t *testing.T) {
	suite.Run(t, new(SwitchToMCPSuite))
}

func (s *SwitchToMCPSuite) TestMapsAllFields() {
	manufacturer := "Gateron"
	pins := 5
	lubed := true
	stem := "POM"
	actuation := 50.0
	springMaterial := "steel"
	vendor := "Divinikey"
	price := 65.0
	orderDate := "2026-01-15"
	deliveryDate := "2026-01-22"
	estimatedDeliveryDate := "2026-03-31"
	status := "Delivered"
	quantity := 90
	notes := "smooth"

	sw := repository.Switch{
		ID:           "sw-1",
		Brand:        "Gateron",
		Manufacturer: &manufacturer,
		Name:         "Oil King",
		Type:         "linear",
		Pins:         &pins,
		FactoryLubed: &lubed,
		Material:     repository.SwitchMaterial{Stem: &stem},
		Force:        repository.SwitchForce{Actuation: &actuation},
		Spring:       repository.SwitchSpring{Material: &springMaterial},
		Purchases: repository.BulkPurchases{{
			Vendor: &vendor, Price: &price,
			OrderDate: &orderDate, DeliveryDate: &deliveryDate, EstimatedDeliveryDate: &estimatedDeliveryDate, OrderStatus: &status,
			Quantity: &quantity,
		}},
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
	}

	out := Switch{}.ToMCP(sw, true, repository.ProfilePreferences{})

	s.Equal("sw-1", out.ID)
	s.Require().NotNil(out.Manufacturer)
	s.Equal("Gateron", *out.Manufacturer)
	s.Require().NotNil(out.Pins)
	s.Equal(5, *out.Pins)
	s.Require().NotNil(out.FactoryLubed)
	s.True(*out.FactoryLubed)
	s.Require().NotNil(out.Material)
	s.Equal("POM", *out.Material.Stem)
	s.Require().NotNil(out.Force)
	s.InDelta(50.0, *out.Force.Actuation, 0.001)
	s.Require().NotNil(out.Spring)
	s.Equal("steel", *out.Spring.Material)
	s.Require().Len(out.Purchases, 1)
	s.Equal(90, *out.Purchases[0].Quantity)
	s.Require().NotNil(out.Purchases[0].OrderDate)
	s.Equal("2026-01-15", *out.Purchases[0].OrderDate)
	s.Require().NotNil(out.Purchases[0].DeliveryDate)
	s.Equal("2026-01-22", *out.Purchases[0].DeliveryDate)
	s.Require().NotNil(out.Purchases[0].EstimatedDeliveryDate)
	s.Equal("2026-03-31", *out.Purchases[0].EstimatedDeliveryDate)
	s.Require().NotNil(out.Purchases[0].OrderStatus)
	s.Equal("Delivered", *out.Purchases[0].OrderStatus)
	s.Require().NotNil(out.Notes)
	s.Equal("smooth", *out.Notes)
	s.Require().NotNil(out.Visibility)
	s.Equal("public", *out.Visibility)
}

func (s *SwitchToMCPSuite) TestNonOwner_OmitsVisibility() {
	out := Switch{}.ToMCP(repository.Switch{ID: "sw-1", Visibility: repository.VisibilityPublic}, false, repository.ProfilePreferences{})

	s.Nil(out.Visibility)
}

// An all-unset group collapses to nil so it's omitted from the tool result
// entirely, rather than appearing as an object of zero values.
func (s *SwitchToMCPSuite) TestEmptyGroups_CollapseToNil() {
	out := Switch{}.ToMCP(repository.Switch{ID: "sw-1", Visibility: repository.VisibilityPrivate}, true, repository.ProfilePreferences{})

	s.Nil(out.Material)
	s.Nil(out.Force)
	s.Nil(out.Spring)
	s.Nil(out.Purchases)
	s.Nil(out.FactoryLubed)
	s.Nil(out.Manufacturer)
	s.Nil(out.Pins)
	s.Nil(out.Notes)
}

// One set field is enough to keep the whole group, so a partially recorded
// group isn't silently dropped.
func (s *SwitchToMCPSuite) TestPartiallySetGroup_IsRetained() {
	bottomOut := 62.0

	out := Switch{}.ToMCP(repository.Switch{
		Force: repository.SwitchForce{BottomOut: &bottomOut},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Force)
	s.Require().NotNil(out.Force.BottomOut)
	s.InDelta(62.0, *out.Force.BottomOut, 0.001)
	s.Nil(out.Force.Actuation)
}

func (s *SwitchToMCPSuite) TestOnlySpringLengthSet_SpringRetained() {
	length := 18.0

	out := Switch{}.ToMCP(repository.Switch{
		Spring: repository.SwitchSpring{Length: &length},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Spring)
	s.Require().NotNil(out.Spring.Length)
	s.InDelta(18.0, *out.Spring.Length, 0.001)
}

// A recorded zero (a free purchase, a 0-pin count) must stay distinct from
// an unset field, or MCP would report "not recorded" for a switch REST
// reports as 0.
func (s *SwitchToMCPSuite) TestRecordedZero_SurvivesRoundTrip() {
	pins := 0
	price := 0.0
	quantity := 0

	out := Switch{}.ToMCP(repository.Switch{
		Pins:      &pins,
		Purchases: repository.BulkPurchases{{Price: &price, Quantity: &quantity}},
	}, true, repository.ProfilePreferences{Currency: "USD"})

	s.Require().NotNil(out.Pins)
	s.Zero(*out.Pins)
	s.Require().Len(out.Purchases, 1)
	s.Require().NotNil(out.Purchases[0].Price)
	s.Zero(*out.Purchases[0].Price)
	s.Require().NotNil(out.Purchases[0].Quantity)
	s.Zero(*out.Purchases[0].Quantity)

	raw, err := json.Marshal(out)
	s.Require().NoError(err)
	s.JSONEq(
		`{"id":"","brand":"","name":"","type":"","pins":0,"purchases":[{"price":0,"currency":"USD","quantity":0}],"total_quantity":0,"total_cost":0,"currency":"USD","visibility":"","has_image":false}`,
		string(raw),
	)
}

func (s *SwitchToMCPSuite) TestNoImagePath_HasImageFalse() {
	out := Switch{}.ToMCP(repository.Switch{}, true, repository.ProfilePreferences{})

	s.False(out.HasImage)
}

func (s *SwitchToMCPSuite) TestImagePathSet_HasImageTrue() {
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")

	out := Switch{}.ToMCP(repository.Switch{ImagePath: &key}, true, repository.ProfilePreferences{})

	s.True(out.HasImage)
}

func (s *SwitchToMCPSuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Divinikey"
	status := "Delivered"
	price := 65.0

	out := Switch{}.ToMCP(repository.Switch{
		ID:         "sw-1",
		Purchases:  repository.BulkPurchases{{Vendor: &vendor, OrderStatus: &status, Price: &price}},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Require().Len(out.Purchases, 1)
	s.Nil(out.Purchases[0].Price)
	s.Equal(&vendor, out.Purchases[0].Vendor)
	s.Equal(&status, out.Purchases[0].OrderStatus)
	s.Nil(out.Purchases[0].Currency)
}

func (s *SwitchToMCPSuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	price := 65.0

	out := Switch{}.ToMCP(repository.Switch{
		ID:         "sw-1",
		Purchases:  repository.BulkPurchases{{Price: &price}},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().Len(out.Purchases, 1)
	s.Require().NotNil(out.Purchases[0].Price)
	s.InDelta(price, *out.Purchases[0].Price, 0.0001)
	s.Require().NotNil(out.Purchases[0].Currency)
	s.Equal("EUR", *out.Purchases[0].Currency)
}

func (s *SwitchToMCPSuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	price := 65.0

	out := Switch{}.ToMCP(repository.Switch{
		ID:         "sw-1",
		Purchases:  repository.BulkPurchases{{Price: &price}},
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})

	s.Require().Len(out.Purchases, 1)
	s.Require().NotNil(out.Purchases[0].Price)
	s.InDelta(price, *out.Purchases[0].Price, 0.0001)
	s.Require().NotNil(out.Purchases[0].Currency)
	s.Equal("EUR", *out.Purchases[0].Currency)
}

type SwitchToMCPSummarySuite struct {
	suite.Suite
}

func TestSwitchToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(SwitchToMCPSummarySuite))
}

func (s *SwitchToMCPSummarySuite) TestMapsSummaryFields() {
	notes := "should not appear"
	orderStatus := "Delivered"

	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:        "sw-1",
		Brand:     "Gateron",
		Name:      "Oil King",
		Type:      "linear",
		Notes:     &notes,
		Purchases: repository.BulkPurchases{{OrderStatus: &orderStatus}},
	}, true, repository.ProfilePreferences{})

	s.Equal("sw-1", out.ID)
	s.Equal("Gateron", out.Brand)
	s.Equal("Oil King", out.Name)
	s.Equal("linear", out.Type)
	s.Equal(&orderStatus, out.OrderStatus)
	s.False(out.HasImage)
}

func (s *SwitchToMCPSummarySuite) TestImagePathSet_HasImageTrue() {
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")

	out := Switch{}.ToMCPSummary(repository.Switch{ImagePath: &key}, true, repository.ProfilePreferences{})

	s.True(out.HasImage)
}

func (s *SwitchToMCPSummarySuite) TestOwnerShowPriceToMeTrue_IncludesPrice() {
	price := 8.50

	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:        "sw-1",
		Purchases: repository.BulkPurchases{{Price: &price}},
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true})

	s.Require().NotNil(out.Price)
	s.InDelta(price, *out.Price, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *SwitchToMCPSummarySuite) TestOwnerShowPriceToMeFalse_OmitsPrice() {
	price := 8.50

	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:        "sw-1",
		Purchases: repository.BulkPurchases{{Price: &price}},
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Nil(out.Price)
	s.Nil(out.Currency)
}

func (s *SwitchToMCPSummarySuite) TestNonOwnerShowPriceToOthersFalse_OmitsPrice() {
	price := 8.50

	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:        "sw-1",
		Purchases: repository.BulkPurchases{{Price: &price}},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Nil(out.Price)
	s.Nil(out.Currency)
}

func (s *SwitchToMCPSummarySuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	price := 8.50

	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:        "sw-1",
		Purchases: repository.BulkPurchases{{Price: &price}},
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().NotNil(out.Price)
	s.InDelta(price, *out.Price, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

type SwitchFromMCPSuite struct {
	suite.Suite
}

func TestSwitchFromMCPSuite(t *testing.T) {
	suite.Run(t, new(SwitchFromMCPSuite))
}

func (s *SwitchFromMCPSuite) TestMapsAllFields() {
	stem := "POM"
	actuation := 50.0
	springLength := 22.0
	vendor := "Divinikey"
	price := 0.0
	orderDate := "2026-01-15"
	deliveryDate := "2026-01-22"
	estimatedDeliveryDate := "2026-03-31"
	status := "Delivered"

	out := Switch{}.FromMCP(schema.SwitchInput{
		Brand:    "Gateron",
		Name:     "Oil King",
		Type:     "linear",
		Material: &schema.SwitchMaterial{Stem: &stem},
		Force:    &schema.SwitchForce{Actuation: &actuation},
		Spring:   &schema.SwitchSpring{Length: &springLength},
		Purchases: []schema.SwitchPurchaseInput{{
			Vendor: &vendor, Price: &price,
			OrderDate: &orderDate, DeliveryDate: &deliveryDate, EstimatedDeliveryDate: &estimatedDeliveryDate, OrderStatus: &status,
		}},
		Visibility: "public",
	})

	s.Equal("Gateron", out.Brand)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Require().NotNil(out.Material.Stem)
	s.Equal("POM", *out.Material.Stem)
	s.Require().NotNil(out.Force.Actuation)
	s.InDelta(50.0, *out.Force.Actuation, 0.001)
	s.Require().NotNil(out.Spring.Length)
	s.InDelta(22.0, *out.Spring.Length, 0.001)
	s.Require().NotNil(out.Purchases[0].Price)
	s.Zero(*out.Purchases[0].Price, "a recorded zero price must survive the inbound mapping too")
	s.Require().NotNil(out.Purchases[0].OrderDate)
	s.Equal("2026-01-15", *out.Purchases[0].OrderDate)
	s.Require().NotNil(out.Purchases[0].DeliveryDate)
	s.Equal("2026-01-22", *out.Purchases[0].DeliveryDate)
	s.Require().NotNil(out.Purchases[0].EstimatedDeliveryDate)
	s.Equal("2026-03-31", *out.Purchases[0].EstimatedDeliveryDate)
	s.Require().NotNil(out.Purchases[0].OrderStatus)
	s.Equal("Delivered", *out.Purchases[0].OrderStatus)
}

// ID and UserID are set by the caller and the repository layer
// respectively, never by the tool argument.
func (s *SwitchFromMCPSuite) TestLeavesIdentityUnset() {
	out := Switch{}.FromMCP(schema.SwitchInput{Brand: "B", Name: "N", Type: "linear", Visibility: "private"})

	s.Empty(out.ID)
	s.Empty(out.UserID)
}

func (s *SwitchFromMCPSuite) TestNilGroups_MapToZeroValues() {
	out := Switch{}.FromMCP(schema.SwitchInput{Brand: "B", Name: "N", Type: "linear", Visibility: "private"})

	s.Nil(out.Material.Stem)
	s.Nil(out.Force.Actuation)
	s.Nil(out.Spring.Material)
	s.Nil(out.Purchases)
}

func (s *SwitchToMCPSummarySuite) TestOwner_IncludesVisibility() {
	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:         "sw-1",
		Visibility: repository.VisibilityPrivate,
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Visibility)
	s.Equal("private", *out.Visibility)
}

func (s *SwitchToMCPSummarySuite) TestNonOwner_OmitsVisibility() {
	out := Switch{}.ToMCPSummary(repository.Switch{
		ID:         "sw-1",
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Nil(out.Visibility)
}

func (s *SwitchToMCPSummarySuite) TestSeveralPurchases_AggregatesStatusAndPrice() {
	out := Switch{}.ToMCPSummary(repository.Switch{
		ID: "sw-1",
		Purchases: repository.BulkPurchases{
			{OrderStatus: new("Delivered"), Price: new(36.0)},
			{OrderStatus: new("Ordered"), Price: new(54.0)},
		},
	}, true, repository.ProfilePreferences{ShowPriceToMe: true})

	s.Equal(new("Ordered"), out.OrderStatus)
	s.Equal(new(90.0), out.Price)
}

func (s *SwitchToMCPSuite) TestSeveralPurchases_MapsEachAndTotalsQuantity() {
	out := Switch{}.ToMCP(repository.Switch{
		ID:        "sw-1",
		Purchases: repository.BulkPurchases{{Vendor: new("CannonKeys"), Quantity: new(70)}, {Vendor: new("Divinikey"), Quantity: new(35)}},
	}, true, repository.ProfilePreferences{})

	s.Equal([]schema.SwitchPurchase{{Vendor: new("CannonKeys"), Quantity: new(70)}, {Vendor: new("Divinikey"), Quantity: new(35)}}, out.Purchases)
	s.Equal(new(105), out.TotalQuantity)
}

func (s *SwitchFromMCPSuite) TestSeveralPurchases_MapsEach() {
	out := Switch{}.FromMCP(schema.SwitchInput{
		Brand: "B", Name: "N", Type: "linear", Visibility: "private",
		Purchases: []schema.SwitchPurchaseInput{{Vendor: new("CannonKeys")}, {Vendor: new("Divinikey"), Quantity: new(35)}},
	})

	s.Equal(repository.BulkPurchases{{Vendor: new("CannonKeys")}, {Vendor: new("Divinikey"), Quantity: new(35)}}, out.Purchases)
}
