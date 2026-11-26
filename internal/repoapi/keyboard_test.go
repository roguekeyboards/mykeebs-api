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

func fullRepoKeyboard() repository.Keyboard {
	return repository.Keyboard{
		UserID: "alice",
		ID:     "kb1",
		Brand:  "Keychron",
		Name:   "Q1",
		Size:   strPtr("75%"),
		Layout: strPtr("WK"),
		Design: repository.KeyboardDesign{
			TopCase:    repository.KeyboardMaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			BottomCase: repository.KeyboardMaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			Weight:     repository.KeyboardMaterialColor{Material: strPtr("Brass"), Color: strPtr("Gold")},
		},
		Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{
			{ID: "p1", Material: "FR4"},
			{
				ID: "p2", Material: "PC", Color: strPtr("Clear"), Thickness: floatPtr(1.2),
				Purchase: repository.KeyboardPurchase{Vendor: strPtr("Keychron"), Price: floatPtr(30), OrderDate: strPtr("2026-02-01")},
			},
		}),
		PCBs: repository.KeyboardPCBsMap([]repository.KeyboardPCB{{
			ID:           "b1",
			Thickness:    floatPtr(1.6),
			Firmware:     strPtr("QMK/VIA"),
			Assembly:     strPtr("Hot-swap"),
			Connectivity: strPtr("Wired"),
		}}),
		Purchase: repository.KeyboardPurchase{
			Vendor:                strPtr("Keychron"),
			Price:                 floatPtr(199.99),
			OrderDate:             strPtr("2026-01-15"),
			DeliveryDate:          strPtr("2026-01-22"),
			EstimatedDeliveryDate: strPtr("2026-03-31"),
			OrderStatus:           strPtr("Delivered"),
		},
		Notes:      strPtr("stock lubed"),
		Visibility: repository.VisibilityPrivate,
	}
}

type KeyboardToAPISuite struct {
	suite.Suite
}

func TestKeyboardToAPISuite(t *testing.T) {
	suite.Run(t, new(KeyboardToAPISuite))
}

func (s *KeyboardToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	kb := fullRepoKeyboard()
	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Equal(kb.ID, out.Id)
	s.Equal(kb.Brand, out.Brand)
	s.Equal(kb.Name, out.Name)
	s.Equal(kb.Size, out.Size)
	s.Equal(kb.Layout, out.Layout)
	s.Equal(kb.Notes, out.Notes)
	s.Require().NotNil(out.Visibility)
	s.Equal(api.Visibility(kb.Visibility), *out.Visibility)

	if s.NotNil(out.Design) {
		s.Require().NotNil(out.Design.TopCase)
		s.Equal(kb.Design.TopCase.Material, out.Design.TopCase.Material)
		s.Equal(kb.Design.TopCase.Color, out.Design.TopCase.Color)
		s.Require().NotNil(out.Design.BottomCase)
		s.Equal(kb.Design.BottomCase.Material, out.Design.BottomCase.Material)
		s.Require().NotNil(out.Design.Weight)
		s.Equal(kb.Design.Weight.Material, out.Design.Weight.Material)
	}
	if s.NotNil(out.Plates) && s.Len(*out.Plates, 2) {
		s.Equal(api.KeyboardPlate{Id: "p1", Material: "FR4"}, (*out.Plates)[0])
		p := (*out.Plates)[1]
		s.Equal("p2", p.Id)
		s.Equal("PC", p.Material)
		s.Equal(strPtr("Clear"), p.Color)
		s.Equal(floatPtr(1.2), p.Thickness)
		if s.NotNil(p.Purchase) {
			s.Equal(strPtr("Keychron"), p.Purchase.Vendor)
			s.Equal(floatPtr(30), p.Purchase.Price)
			s.Require().NotNil(p.Purchase.OrderDate)
			s.Equal("2026-02-01", p.Purchase.OrderDate.Format(dateLayout))
		}
	}
	if s.NotNil(out.Pcbs) && s.Len(*out.Pcbs, 1) {
		pcb := (*out.Pcbs)[0]
		s.Equal("b1", pcb.Id)
		s.Equal(kb.PCBs["b1"].Thickness, pcb.Thickness)
		s.Equal(kb.PCBs["b1"].Firmware, pcb.Firmware)
		s.Equal(kb.PCBs["b1"].Assembly, pcb.Assembly)
		s.Equal(kb.PCBs["b1"].Connectivity, pcb.Connectivity)
		s.Nil(pcb.Purchase)
	}
	s.Equal(floatPtr(229.99), out.TotalCost)
	if s.NotNil(out.Purchase) {
		s.Equal(kb.Purchase.Vendor, out.Purchase.Vendor)
		s.Equal(kb.Purchase.Price, out.Purchase.Price)
		s.Equal(kb.Purchase.OrderStatus, out.Purchase.OrderStatus)
		s.Require().NotNil(out.Purchase.OrderDate)
		s.Equal(*kb.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
		s.Require().NotNil(out.Purchase.DeliveryDate)
		s.Equal(*kb.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
		s.Require().NotNil(out.Purchase.EstimatedDeliveryDate)
		s.Equal(*kb.Purchase.EstimatedDeliveryDate, out.Purchase.EstimatedDeliveryDate.Format(dateLayout))
	}
}

func (s *KeyboardToAPISuite) TestNonOwner_OmitsVisibility() {
	kb := repository.Keyboard{ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPublic}

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Visibility)
}

func (s *KeyboardToAPISuite) TestAllOptionalFieldsNil_SubStructsOmitted() {
	kb := repository.Keyboard{ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate}

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Size)
	s.Nil(out.Layout)
	s.Nil(out.Notes)
	s.Nil(out.Design, "an all-nil KeyboardDesign must map to a nil pointer, not an empty object")
	s.Nil(out.Plates)
	s.Nil(out.Pcbs)
	s.Nil(out.Purchase, "an all-nil KeyboardPurchase must map to a nil pointer, not an empty object")
	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeyboardToAPISuite) TestOneFieldSetInSubStruct_SubStructPresent() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Design: repository.KeyboardDesign{TopCase: repository.KeyboardMaterialColor{Material: strPtr("Aluminum")}},
	}

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	if s.NotNil(out.Design) {
		s.Require().NotNil(out.Design.TopCase)
		s.Equal(strPtr("Aluminum"), out.Design.TopCase.Material)
		s.Nil(out.Design.BottomCase)
		s.Nil(out.Design.Weight)
	}
}

func (s *KeyboardToAPISuite) TestMalformedStoredPartDate_ReturnsError() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		PCBs: repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b1", Purchase: repository.KeyboardPurchase{DeliveryDate: strPtr("not-a-date")}}}),
	}

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	_, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})

	s.Require().Error(err)
}

func (s *KeyboardToAPISuite) TestMalformedStoredDate_ReturnsError() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Purchase: repository.KeyboardPurchase{OrderDate: strPtr("not-a-date")},
	}

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	_, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})

	s.Require().Error(err)
}

func (s *KeyboardToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	kb := fullRepoKeyboard()

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, false, repository.ProfilePreferences{ShowPriceToOthers: false})
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Require().NotNil(out.Plates)
	s.Require().NotNil((*out.Plates)[1].Purchase)
	s.Nil((*out.Plates)[1].Purchase.Price)
	s.Equal(strPtr("Keychron"), (*out.Plates)[1].Purchase.Vendor)
	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(kb.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(kb.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*kb.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*kb.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	s.Nil(out.Purchase.Currency)
}

func (s *KeyboardToAPISuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	kb := fullRepoKeyboard()

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(kb.Purchase.Price, out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
	s.Equal(floatPtr(229.99), out.TotalCost)
	s.Equal(strPtr("EUR"), out.Currency)
}

func (s *KeyboardToAPISuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	kb := fullRepoKeyboard()

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(kb.Purchase.Price, out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

func (s *KeyboardToAPISuite) TestImagesPresent_SignsEachAndPreservesOrder() {
	kb := fullRepoKeyboard()
	img1 := repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")
	img2 := repository.KeyboardImageKey("keyboards/alice/kb1/images/img2")
	kb.Images = map[string]repository.KeyboardImageEntry{
		"img1": {Path: img1, Seq: 0},
		"img2": {Path: img2, Seq: 1},
	}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().SignGetKeyboardImage(mock.Anything, img1).Return("https://example.com/img1", nil)
	images.EXPECT().SignGetKeyboardImage(mock.Anything, img2).Return("https://example.com/img2", nil)
	kr := Keyboard{Images: images}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Require().Len(*out.Images, 2)
	s.Equal("img1", (*out.Images)[0].ImageId)
	s.Equal("https://example.com/img1", *(*out.Images)[0].Url)
	s.Equal("img2", (*out.Images)[1].ImageId)
	s.Equal("https://example.com/img2", *(*out.Images)[1].Url)
}

func (s *KeyboardToAPISuite) TestNoImages_ImagesFieldNil() {
	kb := fullRepoKeyboard()

	kr := Keyboard{Images: mocks.NewMockKeyboardImageStore(s.T())}
	out, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Images)
}

func (s *KeyboardToAPISuite) TestImageSignError_Propagates() {
	kb := fullRepoKeyboard()
	imgPath := repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")
	kb.Images = map[string]repository.KeyboardImageEntry{"img1": {Path: imgPath}}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().SignGetKeyboardImage(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	kr := Keyboard{Images: images}
	_, err := kr.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})

	s.Require().Error(err)
}

type KeyboardToRepoSuite struct {
	suite.Suite
}

func TestKeyboardToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeyboardToRepoSuite))
}

func (s *KeyboardToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	orderDate := openapi_types.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	deliveryDate := openapi_types.Date{Time: time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)}
	estimatedDeliveryDate := openapi_types.Date{Time: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)}
	in := api.KeyboardInput{
		Brand: "Keychron",
		Name:  "Q1",
		Size:  strPtr("75%"),
		Design: &api.KeyboardDesign{
			TopCase:    &api.MaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			BottomCase: &api.MaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			Weight:     &api.MaterialColor{Material: strPtr("Brass"), Color: strPtr("Gold")},
		},
		Purchase: &api.PurchaseInput{
			Vendor:                strPtr("Keychron"),
			Price:                 floatPtr(199.99),
			OrderDate:             &orderDate,
			DeliveryDate:          &deliveryDate,
			EstimatedDeliveryDate: &estimatedDeliveryDate,
			OrderStatus:           strPtr("Delivered"),
		},
		Notes:      strPtr("stock lubed"),
		Visibility: api.VisibilityPrivate,
	}

	kb := Keyboard{}.ToRepo(in)

	s.Empty(kb.UserID, "ToRepo must not set UserID - that's the handler's job")
	s.Empty(kb.ID, "ToRepo must not set ID - that's the handler's job")
	s.Equal(in.Brand, kb.Brand)
	s.Equal(in.Name, kb.Name)
	s.Equal(in.Size, kb.Size)
	s.Equal(repository.Visibility(in.Visibility), kb.Visibility)

	s.Equal(in.Design.TopCase.Material, kb.Design.TopCase.Material)
	s.Equal(in.Design.BottomCase.Material, kb.Design.BottomCase.Material)
	s.Equal(in.Design.Weight.Material, kb.Design.Weight.Material)

	s.Equal(in.Purchase.Vendor, kb.Purchase.Vendor)
	s.Equal(in.Purchase.Price, kb.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, kb.Purchase.OrderStatus)
	s.Require().NotNil(kb.Purchase.OrderDate)
	s.Equal(in.Purchase.OrderDate.Format(dateLayout), *kb.Purchase.OrderDate)
	s.Require().NotNil(kb.Purchase.DeliveryDate)
	s.Equal(in.Purchase.DeliveryDate.Format(dateLayout), *kb.Purchase.DeliveryDate)
	s.Require().NotNil(kb.Purchase.EstimatedDeliveryDate)
	s.Equal(in.Purchase.EstimatedDeliveryDate.Format(dateLayout), *kb.Purchase.EstimatedDeliveryDate)
}

func (s *KeyboardToRepoSuite) TestPlateToRepo_MapsAllFields() {
	orderDate := openapi_types.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}

	plate := Keyboard{}.PlateToRepo(api.KeyboardPlateInput{
		Material: "PC", Color: strPtr("Clear"), Thickness: floatPtr(1.2),
		Purchase: &api.PurchaseInput{Price: floatPtr(30), OrderDate: &orderDate},
	})

	s.Equal(repository.KeyboardPlate{
		Material: "PC", Color: strPtr("Clear"), Thickness: floatPtr(1.2),
		Purchase: repository.KeyboardPurchase{Price: floatPtr(30), OrderDate: strPtr("2026-01-15")},
	}, plate)
}

func (s *KeyboardToRepoSuite) TestPCBToRepo_MapsAllFields() {
	pcb := Keyboard{}.PCBToRepo(api.KeyboardPCBInput{
		Thickness: floatPtr(1.6), Firmware: strPtr("QMK/VIA"), Assembly: strPtr("Hotswap"), Connectivity: strPtr("Wired"),
	})

	s.Equal(repository.KeyboardPCB{
		Thickness: floatPtr(1.6), Firmware: strPtr("QMK/VIA"), Assembly: strPtr("Hotswap"), Connectivity: strPtr("Wired"),
	}, pcb)
}

func (s *KeyboardToRepoSuite) TestNilSubStructs_ProduceZeroValueStructs() {
	in := api.KeyboardInput{Brand: "Keychron", Name: "Q1", Visibility: api.VisibilityPrivate}

	kb := Keyboard{}.ToRepo(in)

	s.Equal(repository.KeyboardDesign{}, kb.Design)
	s.Nil(kb.Plates)
	s.Nil(kb.PCBs)
	s.Equal(repository.KeyboardPurchase{}, kb.Purchase)
}

func (s *KeyboardToAPISuite) TestStripPrices_ClearsPriceAndCurrencyKeepsRest() {
	price, currency, vendor := 199.99, "EUR", "Amazon"
	out := api.Keyboard{Purchase: &api.Purchase{Price: &price, Currency: &currency, Vendor: &vendor}}

	Keyboard{}.StripPrices(&out)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Nil(out.Purchase.Currency)
	s.Equal(&vendor, out.Purchase.Vendor)
}

func (s *KeyboardToAPISuite) TestStripPrices_OnlyPriceSet_DropsPurchase() {
	price, currency := 199.99, "EUR"
	out := api.Keyboard{Purchase: &api.Purchase{Price: &price, Currency: &currency}}

	Keyboard{}.StripPrices(&out)

	s.Nil(out.Purchase)
}

func (s *KeyboardToAPISuite) TestStripPrices_ClearsTotalCostAndPartPrices() {
	price, currency, vendor := 30.0, "EUR", "Amazon"
	out := api.Keyboard{
		TotalCost: &price,
		Currency:  &currency,
		Plates:    &[]api.KeyboardPlate{{Id: "p1", Purchase: &api.Purchase{Price: &price, Currency: &currency, Vendor: &vendor}}},
		Pcbs:      &[]api.KeyboardPCB{{Id: "b1", Purchase: &api.Purchase{Price: &price, Currency: &currency}}},
	}

	Keyboard{}.StripPrices(&out)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Equal(&api.Purchase{Vendor: &vendor}, (*out.Plates)[0].Purchase)
	s.Nil((*out.Pcbs)[0].Purchase)
}

func (s *KeyboardToAPISuite) TestStripPrices_NoPurchase_NoOp() {
	out := api.Keyboard{Id: "kb1"}

	Keyboard{}.StripPrices(&out)

	s.Equal(api.Keyboard{Id: "kb1"}, out)
}

func (s *KeyboardToAPISuite) TestParts_ListedInSeqOrder() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "B", Name: "N", Visibility: repository.VisibilityPrivate,
		Plates: map[string]repository.KeyboardPlate{
			"z": {ID: "z", Material: "AL", Seq: 1},
			"a": {ID: "a", Material: "PC", Seq: 2},
			"m": {ID: "m", Material: "PP", Seq: 0},
		},
		PCBs: map[string]repository.KeyboardPCB{
			"y": {ID: "y", Seq: 2},
			"b": {ID: "b", Seq: 1},
		},
	}

	out, err := Keyboard{}.ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Plates)
	s.Equal([]string{"m", "z", "a"}, []string{(*out.Plates)[0].Id, (*out.Plates)[1].Id, (*out.Plates)[2].Id})
	s.Require().NotNil(out.Pcbs)
	s.Equal([]string{"b", "y"}, []string{(*out.Pcbs)[0].Id, (*out.Pcbs)[1].Id})
}
