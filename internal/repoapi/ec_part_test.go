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

type ECPartSuite struct {
	suite.Suite
}

func TestECPartSuite(t *testing.T) {
	suite.Run(t, new(ECPartSuite))
}

func (s *ECPartSuite) TestToAPI_MapsSubObjectAndPurchase() {
	p := repository.ECPart{
		ID: "p1", Brand: "DynaCap", Name: "Sliders", PartType: repository.ECPartTypeSliders,
		Sliders: &repository.ECSliders{Size: new(repository.ECPartSize2U), Stem: new(repository.ECSliderStemOEM), Material: new("POM")},
		Purchases: repository.BulkPurchases{{
			Vendor: new("Omnitype"), Price: new(20.0), Quantity: new(72),
			OrderDate: new("2026-09-21"), DeliveryDate: new("2026-09-25"), OrderStatus: new("Delivered"),
		}},
		Notes:      new("smooth"),
		Visibility: repository.VisibilityPublic,
	}

	out, err := ECPart{}.ToAPI(s.T().Context(), p, true, repository.ProfilePreferences{Currency: "USD"})

	purchase := api.ECPartPurchase{
		Vendor: new("Omnitype"), Price: new(20.0), Currency: new("USD"), Quantity: new(72),
		OrderDate:    &openapi_types.Date{Time: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)},
		DeliveryDate: &openapi_types.Date{Time: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
		OrderStatus:  new("Delivered"),
	}
	s.Require().NoError(err)
	s.Equal(api.ECPart{
		Id: "p1", Brand: "DynaCap", Name: "Sliders", PartType: api.ECPartTypeSliders,
		Sliders:       &api.ECSliders{Size: new(api.ECPartSizeN2U), Stem: new(api.ECSliderStemOEM), Material: new("POM")},
		Purchase:      &purchase,
		Purchases:     &[]api.ECPartPurchase{purchase},
		TotalQuantity: new(72),
		Notes:         new("smooth"),
		Visibility:    new(api.VisibilityPublic),
	}, out)
}

func (s *ECPartSuite) TestToAPI_EmptyPurchase_Omitted() {
	out, err := ECPart{}.ToAPI(s.T().Context(), repository.ECPart{ID: "p1", PartType: repository.ECPartTypeDomes}, true, repository.ProfilePreferences{})

	s.Require().NoError(err)
	s.Nil(out.Purchase)
	s.Nil(out.Domes)
}

func (s *ECPartSuite) TestToAPI_NonOwner_HidesVisibilityAndPriceUnlessShared() {
	p := repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic, Purchases: repository.BulkPurchases{{Price: new(36.0)}}}

	hidden, err := ECPart{}.ToAPI(s.T().Context(), p, false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Nil(hidden.Visibility)
	s.Require().NotNil(hidden.Purchase)
	s.Nil(hidden.Purchase.Price)

	shown, err := ECPart{}.ToAPI(s.T().Context(), p, false, repository.ProfilePreferences{ShowPriceToOthers: true, Currency: "USD"})
	s.Require().NoError(err)
	s.Require().NotNil(shown.Purchase)
	s.Equal(new(36.0), shown.Purchase.Price)
}

func (s *ECPartSuite) TestToAPI_MalformedStoredDate_ReturnsError() {
	_, err := ECPart{}.ToAPI(s.T().Context(), repository.ECPart{Purchases: repository.BulkPurchases{{OrderDate: new("not-a-date")}}}, true, repository.ProfilePreferences{})

	s.Error(err)
}

func (s *ECPartSuite) TestToRepo_MapsEveryField() {
	in := api.ECPartInput{
		Brand: "DynaCap", Name: "Rings", PartType: api.ECPartTypeRings,
		Rings: &api.ECRings{Material: new("Silicone"), Thickness: new(0.5)},
		Purchases: &[]api.ECPartPurchaseInput{
			{
				Vendor: new("Omnitype"), Price: new(6.0), Quantity: new(140),
				OrderDate: &openapi_types.Date{Time: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)},
			},
			{Vendor: new("Omnitype"), Quantity: new(140)},
		},
		Notes:      new("0.5mm"),
		Visibility: api.VisibilityPrivate,
	}

	s.Equal(repository.ECPart{
		Brand: "DynaCap", Name: "Rings", PartType: repository.ECPartTypeRings,
		Rings: &repository.ECRings{Material: new("Silicone"), Thickness: new(0.5)},
		Purchases: repository.BulkPurchases{
			{Vendor: new("Omnitype"), Price: new(6.0), Quantity: new(140), OrderDate: new("2026-09-21")},
			{Vendor: new("Omnitype"), Quantity: new(140)},
		},
		Notes:      new("0.5mm"),
		Visibility: repository.VisibilityPrivate,
	}, ECPart{}.ToRepo(in))
}

func (s *ECPartSuite) TestToRepo_EverySubObject() {
	in := api.ECPartInput{
		Domes:    &api.ECDomes{Weight: new(45.0), Material: new("Silicone"), Color: new("red")},
		Sliders:  &api.ECSliders{Size: new(api.ECPartSizeN1U), Stem: new(api.ECSliderStemMX)},
		Housings: &api.ECHousings{Size: new(api.ECPartSizeN2U), Material: new("POM")},
		Springs:  &api.ECSprings{Material: new("Nickel-Plated")},
	}

	out := ECPart{}.ToRepo(in)

	s.Equal(&repository.ECDomes{Weight: new(45.0), Material: new("Silicone"), Color: new("red")}, out.Domes)
	s.Equal(&repository.ECSliders{Size: new(repository.ECPartSize1U), Stem: new(repository.ECSliderStemMX)}, out.Sliders)
	s.Equal(&repository.ECHousings{Size: new(repository.ECPartSize2U), Material: new("POM")}, out.Housings)
	s.Equal(&repository.ECSprings{Material: new("Nickel-Plated")}, out.Springs)
	s.Nil(out.Rings)
}

func (s *ECPartSuite) TestStripPrices_DropsPurchaseLeftEmpty() {
	out := api.ECPart{Purchase: &api.ECPartPurchase{Price: new(6.0), Currency: new("USD")}}

	ECPart{}.StripPrices(&out)

	s.Nil(out.Purchase)
}

func (s *ECPartSuite) TestStripPrices_KeepsRestOfPurchase() {
	out := api.ECPart{Purchase: &api.ECPartPurchase{Price: new(6.0), Currency: new("USD"), Vendor: new("Omnitype")}}

	ECPart{}.StripPrices(&out)

	s.Equal(&api.ECPartPurchase{Vendor: new("Omnitype")}, out.Purchase)
}

func (s *ECPartSuite) TestToAPI_RejectedImage_OwnerSeesReasonWithoutURL() {
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/u1")
	p := repository.ECPart{ID: "p1", ImagePath: &key, ImageStatus: repository.ImageStatusRejected, ImageRejectionReason: new("Explicit")}

	owner, err := ECPart{}.ToAPI(s.T().Context(), p, true, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Equal(&api.ECPartImage{Status: new(api.ImageStatusRejected), RejectionReason: new("Explicit")}, owner.Image)

	other, err := ECPart{}.ToAPI(s.T().Context(), p, false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Nil(other.Image)
}

func (s *ECPartSuite) TestToAPI_SignFails_ReturnsError() {
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/u1")
	images := mocks.NewMockECPartImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, key).Return("", errors.New("no key"))

	_, err := ECPart{Images: images}.ToAPI(s.T().Context(), repository.ECPart{ImagePath: &key, ImageStatus: repository.ImageStatusApproved}, false, repository.ProfilePreferences{})

	s.Error(err)
}
