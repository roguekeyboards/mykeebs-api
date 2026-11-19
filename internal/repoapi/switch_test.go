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
		},
		Purchase: repository.SwitchPurchase{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    strPtr("2026-01-15"),
			DeliveryDate: strPtr("2026-01-22"),
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		},
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
	}
	if s.NotNil(out.Purchase) {
		s.Equal(sw.Purchase.Vendor, out.Purchase.Vendor)
		s.Equal(sw.Purchase.Price, out.Purchase.Price)
		s.Equal(sw.Purchase.OrderStatus, out.Purchase.OrderStatus)
		s.Equal(sw.Purchase.Quantity, out.Purchase.Quantity)
		s.Require().NotNil(out.Purchase.OrderDate)
		s.Equal(*sw.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
		s.Require().NotNil(out.Purchase.DeliveryDate)
		s.Equal(*sw.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	}
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
	s.Nil(out.Purchase, "an all-nil SwitchPurchase must map to a nil pointer, not an empty object")
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

func (s *SwitchToAPISuite) TestMalformedStoredDate_ReturnsError() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Purchase: repository.SwitchPurchase{OrderDate: strPtr("not-a-date")},
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

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(sw.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(sw.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Equal(sw.Purchase.Quantity, out.Purchase.Quantity)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*sw.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*sw.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	s.Nil(out.Purchase.Currency)
}

func (s *SwitchToAPISuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(sw.Purchase.Price, out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

func (s *SwitchToAPISuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	sw := fullRepoSwitch()

	sr := Switch{Images: mocks.NewMockSwitchImageStore(s.T())}
	out, err := sr.ToAPI(s.T().Context(), sw, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(sw.Purchase.Price, out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
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
		},
		Purchase: &api.SwitchPurchaseInput{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    &orderDate,
			DeliveryDate: &deliveryDate,
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		},
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
	s.Equal(in.Purchase.Vendor, sw.Purchase.Vendor)
	s.Equal(in.Purchase.Price, sw.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, sw.Purchase.OrderStatus)
	s.Equal(in.Purchase.Quantity, sw.Purchase.Quantity)
	s.Require().NotNil(sw.Purchase.OrderDate)
	s.Equal(in.Purchase.OrderDate.Format(dateLayout), *sw.Purchase.OrderDate)
	s.Require().NotNil(sw.Purchase.DeliveryDate)
	s.Equal(in.Purchase.DeliveryDate.Format(dateLayout), *sw.Purchase.DeliveryDate)
}

func (s *SwitchToRepoSuite) TestNilSubStructs_ProduceZeroValueStructs() {
	in := api.SwitchInput{Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.VisibilityPrivate}

	sw := Switch{}.ToRepo(in)

	s.Equal(repository.SwitchMaterial{}, sw.Material)
	s.Equal(repository.SwitchForce{}, sw.Force)
	s.Equal(repository.SwitchSpring{}, sw.Spring)
	s.Equal(repository.SwitchPurchase{}, sw.Purchase)
}

func (s *SwitchToAPISuite) TestStripPrices_ClearsPriceAndCurrencyKeepsRest() {
	price, currency, vendor := 8.5, "EUR", "CannonKeys"
	out := api.Switch{Purchase: &api.SwitchPurchase{Price: &price, Currency: &currency, Vendor: &vendor}}

	Switch{}.StripPrices(&out)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Nil(out.Purchase.Currency)
	s.Equal(&vendor, out.Purchase.Vendor)
}

func (s *SwitchToAPISuite) TestStripPrices_OnlyPriceSet_DropsPurchase() {
	price, currency := 8.5, "EUR"
	out := api.Switch{Purchase: &api.SwitchPurchase{Price: &price, Currency: &currency}}

	Switch{}.StripPrices(&out)

	s.Nil(out.Purchase)
}

func (s *SwitchToAPISuite) TestStripPrices_NoPurchase_NoOp() {
	out := api.Switch{Id: "sw1"}

	Switch{}.StripPrices(&out)

	s.Equal(api.Switch{Id: "sw1"}, out)
}
