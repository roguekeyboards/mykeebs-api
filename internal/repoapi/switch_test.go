package repoapi

import (
	"errors"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

func strPtr(s string) *string     { return &s }
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }

func imageKeyPtr(s string) *repository.KeycapKitImageKey {
	k := repository.KeycapKitImageKey(s)
	return &k
}

func fullRepoSwitch() repository.Switch {
	return repository.Switch{
		UserID:       "alice",
		ID:           "sw1",
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: repository.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: repository.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: repository.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
			Length:      floatPtr(22),
		},
		Purchases: repository.BulkPurchases{{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    strPtr("2026-01-15"),
			DeliveryDate: strPtr("2026-01-22"),
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		}},
		Notes:      strPtr("smooth"),
		Visibility: repository.VisibilityPrivate,
	}
}

type SwitchToAPISuite struct {
	suite.Suite
}

func TestSwitchToAPISuite(t *testing.T) {
	suite.Run(t, new(SwitchToAPISuite))
}

func (s *SwitchToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	sw := fullRepoSwitch()
	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Equal(sw.ID, out.Id)
	s.Equal(sw.Brand, out.Brand)
	s.Equal(sw.Manufacturer, out.Manufacturer)
	s.Equal(sw.Name, out.Name)
	s.Equal(sw.Type, out.Type)
	s.Equal(sw.Pins, out.Pins)
	s.Equal(sw.FactoryLubed, out.FactoryLubed)
	s.Equal(sw.Notes, out.Notes)
	s.Require().NotNil(out.Visibility)
	s.Equal(api.Visibility(sw.Visibility), *out.Visibility)

	if s.NotNil(out.Material) {
		s.Equal(sw.Material.TopHousing, out.Material.TopHousing)
		s.Equal(sw.Material.BottomHousing, out.Material.BottomHousing)
		s.Equal(sw.Material.Stem, out.Material.Stem)
	}
	if s.NotNil(out.Force) {
		s.Equal(sw.Force.Actuation, out.Force.Actuation)
		s.Equal(sw.Force.BottomOut, out.Force.BottomOut)
	}
	if s.NotNil(out.Spring) {
		s.Equal(sw.Spring.Material, out.Spring.Material)
		s.Equal(sw.Spring.PreTravel, out.Spring.PreTravel)
		s.Equal(sw.Spring.TotalTravel, out.Spring.TotalTravel)
		s.Equal(sw.Spring.Length, out.Spring.Length)
	}
	s.Require().NotNil(out.Purchases)
	s.Require().Len(*out.Purchases, 1)
	s.Equal(sw.Purchases[0].Vendor, (*out.Purchases)[0].Vendor)
	s.Equal(sw.Purchases[0].Price, (*out.Purchases)[0].Price)
	s.Equal(sw.Purchases[0].OrderStatus, (*out.Purchases)[0].OrderStatus)
	s.Equal(sw.Purchases[0].Quantity, (*out.Purchases)[0].Quantity)
	s.Require().NotNil((*out.Purchases)[0].OrderDate)
	s.Equal(*sw.Purchases[0].OrderDate, (*out.Purchases)[0].OrderDate.Format(dateLayout))
	s.Require().NotNil((*out.Purchases)[0].DeliveryDate)
	s.Equal(*sw.Purchases[0].DeliveryDate, (*out.Purchases)[0].DeliveryDate.Format(dateLayout))
}

func (s *SwitchToAPISuite) TestNonOwner_OmitsVisibility() {
	sw := repository.Switch{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPublic}

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Visibility)
}

func (s *SwitchToAPISuite) TestAllOptionalFieldsNil_SubStructsOmitted() {
	sw := repository.Switch{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate}

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Manufacturer)
	s.Nil(out.Pins)
	s.Nil(out.FactoryLubed)
	s.Nil(out.Notes)
	s.Nil(out.Material, "an all-nil SwitchMaterial must map to a nil pointer, not an empty object")
	s.Nil(out.Force, "an all-nil SwitchForce must map to a nil pointer, not an empty object")
	s.Nil(out.Spring, "an all-nil SwitchSpring must map to a nil pointer, not an empty object")
	s.Nil(out.Purchases)
}

func (s *SwitchToAPISuite) TestOneFieldSetInSubStruct_SubStructPresent() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Material: repository.SwitchMaterial{Stem: strPtr("POM")},
	}

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	if s.NotNil(out.Material) {
		s.Nil(out.Material.TopHousing)
		s.Nil(out.Material.BottomHousing)
		s.Equal(strPtr("POM"), out.Material.Stem)
	}
}

func (s *SwitchToAPISuite) TestOnlySpringLengthSet_SpringPresent() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Spring: repository.SwitchSpring{Length: floatPtr(18)},
	}

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	if s.NotNil(out.Spring) {
		s.Equal(floatPtr(18), out.Spring.Length)
	}
}

func (s *SwitchToAPISuite) TestMalformedStoredDate_ReturnsError() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Purchases: repository.BulkPurchases{{OrderDate: strPtr("not-a-date")}},
	}

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	_, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})

	s.Require().Error(err)
}

func (s *SwitchToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, false, repository.ProfilePreferences{ShowPriceToOthers: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchases)
	s.Require().Len(*out.Purchases, 1)
	s.Nil((*out.Purchases)[0].Price)
	s.Equal(sw.Purchases[0].Vendor, (*out.Purchases)[0].Vendor)
	s.Equal(sw.Purchases[0].OrderStatus, (*out.Purchases)[0].OrderStatus)
	s.Equal(sw.Purchases[0].Quantity, (*out.Purchases)[0].Quantity)
	s.Require().NotNil((*out.Purchases)[0].OrderDate)
	s.Equal(*sw.Purchases[0].OrderDate, (*out.Purchases)[0].OrderDate.Format(dateLayout))
	s.Require().NotNil((*out.Purchases)[0].DeliveryDate)
	s.Equal(*sw.Purchases[0].DeliveryDate, (*out.Purchases)[0].DeliveryDate.Format(dateLayout))
	s.Nil((*out.Purchases)[0].Currency)
}

func (s *SwitchToAPISuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchases)
	s.Require().Len(*out.Purchases, 1)
	s.Equal(sw.Purchases[0].Price, (*out.Purchases)[0].Price)
	s.Require().NotNil((*out.Purchases)[0].Currency)
	s.Equal("EUR", *(*out.Purchases)[0].Currency)
}

func (s *SwitchToAPISuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchases)
	s.Require().Len(*out.Purchases, 1)
	s.Equal(sw.Purchases[0].Price, (*out.Purchases)[0].Price)
	s.Require().NotNil((*out.Purchases)[0].Currency)
	s.Equal("EUR", *(*out.Purchases)[0].Currency)
}

func (s *SwitchToAPISuite) TestImagePresent_ReturnsSignedURL() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image/up1")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *sw.ImagePath).Return("https://example.com/img", nil)
	sr := Switch{Images: images}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Image)
	s.Equal("https://example.com/img", *out.Image.Url)
}

func (s *SwitchToAPISuite) TestNoImage_ImageFieldNil() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Image)
}

func (s *SwitchToAPISuite) TestImageSignError_Propagates() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image/up1")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *sw.ImagePath).Return("", errors.New("s3: access denied"))

	sr := Switch{Images: images}
	_, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})

	s.Require().Error(err)
}

type SwitchToRepoSuite struct {
	suite.Suite
}

func TestSwitchToRepoSuite(t *testing.T) {
	suite.Run(t, new(SwitchToRepoSuite))
}

func (s *SwitchToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	orderDate := openapi_types.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	deliveryDate := openapi_types.Date{Time: time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)}
	in := api.SwitchInput{
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: &api.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: &api.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: &api.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
			Length:      floatPtr(22),
		},
		Purchases: &[]api.SwitchPurchaseInput{{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    &orderDate,
			DeliveryDate: &deliveryDate,
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		}},
		Notes:      strPtr("smooth"),
		Visibility: api.VisibilityPrivate,
	}

	sw := Switch{}.ToRepo(in)

	s.Empty(sw.UserID, "ToRepo must not set UserID - that's the handler's job")
	s.Empty(sw.ID, "ToRepo must not set ID - that's the handler's job")
	s.Equal(in.Brand, sw.Brand)
	s.Equal(in.Manufacturer, sw.Manufacturer)
	s.Equal(in.Name, sw.Name)
	s.Equal(in.Type, sw.Type)
	s.Equal(in.Pins, sw.Pins)
	s.Equal(in.FactoryLubed, sw.FactoryLubed)
	s.Equal(in.Notes, sw.Notes)
	s.Equal(repository.Visibility(in.Visibility), sw.Visibility)

	s.Equal(in.Material.TopHousing, sw.Material.TopHousing)
	s.Equal(in.Material.BottomHousing, sw.Material.BottomHousing)
	s.Equal(in.Material.Stem, sw.Material.Stem)
	s.Equal(in.Force.Actuation, sw.Force.Actuation)
	s.Equal(in.Force.BottomOut, sw.Force.BottomOut)
	s.Equal(in.Spring.Material, sw.Spring.Material)
	s.Equal(in.Spring.PreTravel, sw.Spring.PreTravel)
	s.Equal(in.Spring.TotalTravel, sw.Spring.TotalTravel)
	s.Equal(in.Spring.Length, sw.Spring.Length)
	s.Equal((*in.Purchases)[0].Vendor, sw.Purchases[0].Vendor)
	s.Equal((*in.Purchases)[0].Price, sw.Purchases[0].Price)
	s.Equal((*in.Purchases)[0].OrderStatus, sw.Purchases[0].OrderStatus)
	s.Equal((*in.Purchases)[0].Quantity, sw.Purchases[0].Quantity)
	s.Require().NotNil(sw.Purchases[0].OrderDate)
	s.Equal((*in.Purchases)[0].OrderDate.Format(dateLayout), *sw.Purchases[0].OrderDate)
	s.Require().NotNil(sw.Purchases[0].DeliveryDate)
	s.Equal((*in.Purchases)[0].DeliveryDate.Format(dateLayout), *sw.Purchases[0].DeliveryDate)
}

func (s *SwitchToRepoSuite) TestNilSubStructs_ProduceZeroValueStructs() {
	in := api.SwitchInput{Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.VisibilityPrivate}

	sw := Switch{}.ToRepo(in)

	s.Equal(repository.SwitchMaterial{}, sw.Material)
	s.Equal(repository.SwitchForce{}, sw.Force)
	s.Equal(repository.SwitchSpring{}, sw.Spring)
	s.Nil(sw.Purchases)
}

func (s *SwitchToAPISuite) TestStripPrices_NoPurchase_NoOp() {
	out := api.Switch{Id: "sw1"}

	Switch{}.StripPrices(&out)

	s.Equal(api.Switch{Id: "sw1"}, out)
}

func (s *SwitchToRepoSuite) TestEmptyPurchasesSent_ClearsPurchases() {
	in := api.SwitchInput{
		Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.VisibilityPrivate,
		Purchases: &[]api.SwitchPurchaseInput{},
	}

	s.Nil(Switch{}.ToRepo(in).Purchases)
}

func (s *SwitchToAPISuite) TestPurchases_KeepsOrderAndTotalsQuantity() {
	sw := repository.Switch{ID: "sw1", Purchases: repository.BulkPurchases{
		{Vendor: strPtr("Undated"), Quantity: intPtr(10)},
		{Vendor: strPtr("Later"), OrderDate: strPtr("2026-11-23"), Quantity: intPtr(70)},
		{Vendor: strPtr("Earlier"), OrderDate: strPtr("2026-03-01"), Quantity: intPtr(70)},
	}}

	out, err := Switch{}.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{})

	s.Require().NoError(err)
	s.Require().NotNil(out.Purchases)
	s.Len(*out.Purchases, 3)
	s.Equal(strPtr("Later"), (*out.Purchases)[1].Vendor)
	s.Equal(intPtr(150), out.TotalQuantity)
}

func (s *SwitchToAPISuite) TestStripPrices_ClearsEveryPurchaseDroppingEmptyOnes() {
	out := api.Switch{Purchases: &[]api.SwitchPurchase{
		{Price: floatPtr(8.5), Currency: strPtr("EUR"), Vendor: strPtr("CannonKeys")},
		{Price: floatPtr(4.0), Currency: strPtr("EUR")},
	}}

	Switch{}.StripPrices(&out)

	s.Equal(&[]api.SwitchPurchase{{Vendor: strPtr("CannonKeys")}}, out.Purchases)
}

func (s *SwitchToAPISuite) TestStripPrices_EveryPurchaseOnlyPriced_DropsPurchases() {
	out := api.Switch{Purchases: &[]api.SwitchPurchase{{Price: floatPtr(8.5), Currency: strPtr("EUR")}}}

	Switch{}.StripPrices(&out)

	s.Nil(out.Purchases)
}

func (s *SwitchToAPISuite) TestPurchases_DerivesOrderStatusAndTotalCostSkippingCancelled() {
	sw := repository.Switch{Purchases: repository.BulkPurchases{
		{Price: floatPtr(36.0), OrderStatus: strPtr("Delivered")},
		{Price: floatPtr(28.8), OrderStatus: strPtr("Ordered")},
		{Price: floatPtr(50.0), OrderStatus: strPtr("Cancelled")},
	}}

	out, err := Switch{}.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{Currency: "USD"})
	s.Require().NoError(err)

	s.Equal(strPtr("Ordered"), out.OrderStatus)
	s.Equal(floatPtr(64.8), out.TotalCost)
	s.Equal(strPtr("USD"), out.Currency)
}

func (s *SwitchToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsTotalCostKeepsOrderStatus() {
	sw := repository.Switch{Purchases: repository.BulkPurchases{{Price: floatPtr(36.0), OrderStatus: strPtr("Delivered")}}}

	out, err := Switch{}.ToAPI(s.T().Context(), sw, false, repository.ProfilePreferences{Currency: "USD"})
	s.Require().NoError(err)

	s.Equal(strPtr("Delivered"), out.OrderStatus)
	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *SwitchToAPISuite) TestStripPrices_ClearsTotalCost() {
	out := api.Switch{TotalCost: floatPtr(64.8), Currency: strPtr("USD"), OrderStatus: strPtr("Ordered")}

	Switch{}.StripPrices(&out)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Equal(strPtr("Ordered"), out.OrderStatus)
}

func (s *SwitchToRepoSuite) TestEmptyPurchaseObjects_Dropped() {
	in := api.SwitchInput{
		Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.VisibilityPrivate,
		Purchases: &[]api.SwitchPurchaseInput{{}, {Vendor: strPtr("CannonKeys")}},
	}

	s.Equal(repository.BulkPurchases{{Vendor: strPtr("CannonKeys")}}, Switch{}.ToRepo(in).Purchases)
}
