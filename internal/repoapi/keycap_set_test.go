package repoapi

import (
	"context"
	"errors"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/availability"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

func fullRepoKeycapSet() repository.KeycapSet {
	return repository.KeycapSet{
		UserID:     "alice",
		ID:         "ks1",
		Brand:      "GMK",
		Name:       "Laser",
		Profile:    strPtr("Cherry"),
		Material:   strPtr("ABS"),
		Notes:      strPtr("group buy"),
		Visibility: repository.VisibilityPrivate,
	}
}

type KeycapSetToAPISuite struct {
	suite.Suite
}

func TestKeycapSetToAPISuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToAPISuite))
}

func (s *KeycapSetToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	ks := fullRepoKeycapSet()
	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Equal(ks.ID, out.Id)
	s.Equal(ks.Brand, out.Brand)
	s.Equal(ks.Name, out.Name)
	s.Equal(ks.Profile, out.Profile)
	s.Equal(ks.Material, out.Material)
	s.Equal(ks.Notes, out.Notes)
	s.Require().NotNil(out.Visibility)
	s.Equal(api.Visibility(ks.Visibility), *out.Visibility)
}

func (s *KeycapSetToAPISuite) TestNonOwner_OmitsVisibility() {
	ks := repository.KeycapSet{ID: "ks1", Brand: "GMK", Name: "Laser", Visibility: repository.VisibilityPublic}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, false, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Nil(out.Visibility)
}

func (s *KeycapSetToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	ks := repository.KeycapSet{ID: "ks1", Brand: "GMK", Name: "Laser", Visibility: repository.VisibilityPrivate}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Nil(out.Profile)
	s.Nil(out.Material)
	s.Nil(out.Notes)
	s.Nil(out.Kits)
}

func (s *KeycapSetToAPISuite) TestKitsPopulated_MapsEachKit() {
	ks := fullRepoKeycapSet()
	ks.Kits = map[string]repository.KeycapKit{
		"kit1": {KitID: "kit1", Name: "Base"},
		"kit2": {KitID: "kit2", Name: "Extension"},
	}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 2)
	s.Equal("kit1", (*out.Kits)[0].KitId)
	s.Equal("Base", (*out.Kits)[0].Name)
	s.Equal("kit2", (*out.Kits)[1].KitId)
	s.Equal("Extension", (*out.Kits)[1].Name)
}

func (s *KeycapSetToAPISuite) TestKits_ListedInSeqOrder() {
	ks := fullRepoKeycapSet()
	ks.Kits = map[string]repository.KeycapKit{
		"aaa": {KitID: "aaa", Name: "Added last", Seq: 20},
		"zzz": {KitID: "zzz", Name: "Added first", Seq: 10},
	}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 2)
	s.Equal("zzz", (*out.Kits)[0].KitId)
	s.Equal("aaa", (*out.Kits)[1].KitId)
}

func (s *KeycapSetToAPISuite) TestKitsWithDifferingOrderStatus_SetsAggregateOrderStatus() {
	ks := fullRepoKeycapSet()
	delivered, ordered := "Delivered", "Ordered"
	ks.Kits = map[string]repository.KeycapKit{
		"kit1": {KitID: "kit1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderStatus: &delivered}},
		"kit2": {KitID: "kit2", Name: "Extension", Purchase: repository.KeycapKitPurchase{OrderStatus: &ordered}},
	}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.OrderStatus)
	s.Equal("Ordered", *out.OrderStatus)
}

func (s *KeycapSetToAPISuite) TestMalformedStoredKitPurchaseDate_ReturnsError() {
	ks := fullRepoKeycapSet()
	ks.Kits = map[string]repository.KeycapKit{
		"kit1": {KitID: "kit1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderDate: strPtr("not-a-date")}},
	}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	_, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{}, availability.Usage{})

	s.Require().Error(err)
}

func (s *KeycapSetToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsKitPriceKeepsRestOfPurchase() {
	ks := fullRepoKeycapSet()
	repoKit := fullRepoKeycapKit()
	ks.Kits = map[string]repository.KeycapKit{repoKit.KitID: repoKit}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *repoKit.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.ToAPI(context.Background(), ks, false, repository.ProfilePreferences{ShowPriceToOthers: false}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 1)
	kit := (*out.Kits)[0]
	s.Require().NotNil(kit.Purchase)
	s.Nil(kit.Purchase.Price)
	s.Equal(repoKit.Purchase.Vendor, kit.Purchase.Vendor)
	s.Equal(repoKit.Purchase.OrderStatus, kit.Purchase.OrderStatus)
}

func (s *KeycapSetToAPISuite) TestNonOwnerShowPriceToOthersTrue_IncludesKitPrice() {
	ks := fullRepoKeycapSet()
	repoKit := fullRepoKeycapKit()
	ks.Kits = map[string]repository.KeycapKit{repoKit.KitID: repoKit}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *repoKit.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.ToAPI(context.Background(), ks, false, repository.ProfilePreferences{ShowPriceToOthers: true}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 1)
	s.Require().NotNil((*out.Kits)[0].Purchase)
	s.Equal(repoKit.Purchase.Price, (*out.Kits)[0].Purchase.Price)
}

func (s *KeycapSetToAPISuite) TestOwner_AlwaysIncludesKitPriceRegardlessOfShowPriceToMe() {
	ks := fullRepoKeycapSet()
	repoKit := fullRepoKeycapKit()
	ks.Kits = map[string]repository.KeycapKit{repoKit.KitID: repoKit}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *repoKit.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{ShowPriceToMe: false}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 1)
	s.Require().NotNil((*out.Kits)[0].Purchase)
	s.Equal(repoKit.Purchase.Price, (*out.Kits)[0].Purchase.Price)
}

func fullAPIKeycapSetInput() api.KeycapSetInput {
	return api.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Laser",
		Profile:    strPtr("Cherry"),
		Material:   strPtr("ABS"),
		Notes:      strPtr("group buy"),
		Visibility: api.Visibility(repository.VisibilityPrivate),
	}
}

func pricedKeycapSet() repository.KeycapSet {
	ks := fullRepoKeycapSet()
	kit1 := fullRepoKeycapKit()
	kit1.ImagePath = nil
	kit2 := fullRepoKeycapKit()
	kit2.KitID = "kit2"
	kit2.ImagePath = nil
	kit2.Purchase.Price = floatPtr(35.00)
	kit3 := fullRepoKeycapKit()
	kit3.KitID = "kit3"
	kit3.ImagePath = nil
	kit3.Purchase.Price = nil
	ks.Kits = map[string]repository.KeycapKit{kit1.KitID: kit1, kit2.KitID: kit2, kit3.KitID: kit3}
	return ks
}

func (s *KeycapSetToAPISuite) TestTotalCost_Owner_SumsKnownKitPricesRegardlessOfShowPriceToMe() {
	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}

	out, err := kr.ToAPI(context.Background(), pricedKeycapSet(), true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(155.00, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *KeycapSetToAPISuite) TestTotalCost_NonOwnerShowPriceToOthersTrue_Included() {
	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}

	out, err := kr.ToAPI(context.Background(), pricedKeycapSet(), false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true}, availability.Usage{})
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(155.00, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *KeycapSetToAPISuite) TestTotalCost_NonOwnerShowPriceToOthersFalse_Omitted() {
	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}

	out, err := kr.ToAPI(context.Background(), pricedKeycapSet(), false, repository.ProfilePreferences{Currency: "EUR"}, availability.Usage{})
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeycapSetToAPISuite) TestTotalCost_NoPricedKits_Omitted() {
	ks := repository.KeycapSet{ID: "ks1", Brand: "GMK", Name: "Laser", Visibility: repository.VisibilityPrivate}
	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}

	out, err := kr.ToAPI(context.Background(), ks, true, repository.ProfilePreferences{Currency: "EUR"}, availability.Usage{})
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *KeycapSetToAPISuite) TestStripPrices_ClearsKitPricesAndTotalCostKeepsRest() {
	price, total, currency, vendor := 120.0, 155.0, "EUR", "NovelKeys"
	kits := []api.KeycapKit{
		{KitId: "kit1", Purchase: &api.Purchase{Price: &price, Currency: &currency, Vendor: &vendor}},
		{KitId: "kit2", Purchase: &api.Purchase{Price: &price, Currency: &currency}},
		{KitId: "kit3"},
	}
	out := api.KeycapSet{Id: "ks1", TotalCost: &total, Currency: &currency, Kits: &kits}

	KeycapSet{}.StripPrices(&out)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Require().NotNil((*out.Kits)[0].Purchase)
	s.Nil((*out.Kits)[0].Purchase.Price)
	s.Nil((*out.Kits)[0].Purchase.Currency)
	s.Equal(&vendor, (*out.Kits)[0].Purchase.Vendor)
	s.Nil((*out.Kits)[1].Purchase, "a purchase left with nothing is dropped")
	s.Nil((*out.Kits)[2].Purchase)
}

func (s *KeycapSetToAPISuite) TestStripPrices_NoKits_ClearsTotalCost() {
	total, currency := 155.0, "EUR"
	out := api.KeycapSet{Id: "ks1", TotalCost: &total, Currency: &currency}

	KeycapSet{}.StripPrices(&out)

	s.Equal(api.KeycapSet{Id: "ks1"}, out)
}

type KeycapSetToRepoSuite struct {
	suite.Suite
}

func TestKeycapSetToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToRepoSuite))
}

func (s *KeycapSetToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIKeycapSetInput()
	out := KeycapSet{}.ToRepo(in)

	s.Equal(in.Brand, out.Brand)
	s.Equal(in.Name, out.Name)
	s.Equal(in.Profile, out.Profile)
	s.Equal(in.Material, out.Material)
	s.Equal(in.Notes, out.Notes)
	s.Equal(repository.Visibility(in.Visibility), out.Visibility)
	s.Empty(out.UserID)
	s.Empty(out.ID)
}

func (s *KeycapSetToRepoSuite) TestAllOptionalFieldsNil_MapsToNil() {
	in := api.KeycapSetInput{Brand: "GMK", Name: "Laser", Visibility: api.Visibility(repository.VisibilityPrivate)}

	out := KeycapSet{}.ToRepo(in)

	s.Nil(out.Profile)
	s.Nil(out.Material)
	s.Nil(out.Notes)
}

func fullRepoKeycapKit() repository.KeycapKit {
	return repository.KeycapKit{
		KitID:     "kit1",
		Name:      "Base",
		ImagePath: imageKeyPtr("keycap-sets/alice/ks1/kits/kit1/image/up1"),
		Purchase: repository.KeycapKitPurchase{
			Vendor:                strPtr("CannonKeys"),
			Price:                 floatPtr(120.00),
			OrderDate:             strPtr("2026-01-15"),
			DeliveryDate:          strPtr("2026-03-01"),
			EstimatedDeliveryDate: strPtr("2026-02-28"),
			OrderStatus:           strPtr("delivered"),
		},
	}
}

type KeycapKitToAPISuite struct {
	suite.Suite
}

func TestKeycapKitToAPISuite(t *testing.T) {
	suite.Run(t, new(KeycapKitToAPISuite))
}

func (s *KeycapKitToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *k.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.KitToAPI(context.Background(), k, true, repository.ProfilePreferences{Currency: "EUR"})
	s.Require().NoError(err)

	s.Equal(k.KitID, out.KitId)
	s.Equal(k.Name, out.Name)
	s.Require().NotNil(out.Image)
	s.Equal("https://example.com/signed-get", *out.Image.Url)
	s.Require().NotNil(out.Purchase)
	s.Equal(k.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(k.Purchase.Price, out.Purchase.Price)
	s.Equal(k.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*k.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*k.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.EstimatedDeliveryDate)
	s.Equal(*k.Purchase.EstimatedDeliveryDate, out.Purchase.EstimatedDeliveryDate.Format(dateLayout))
}

func (s *KeycapKitToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	k := repository.KeycapKit{KitID: "kit1", Name: "Base"}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := kr.KitToAPI(context.Background(), k, true, repository.ProfilePreferences{Currency: "EUR"})
	s.Require().NoError(err)

	s.Nil(out.Purchase)
	s.Nil(out.Image, "no ImagePath set, so images is never called")
}

func (s *KeycapKitToAPISuite) TestMalformedStoredDate_ReturnsError() {
	k := repository.KeycapKit{
		KitID: "kit1", Name: "Base",
		Purchase: repository.KeycapKitPurchase{OrderDate: strPtr("not-a-date")},
	}

	kr := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	_, err := kr.KitToAPI(context.Background(), k, true, repository.ProfilePreferences{Currency: "EUR"})

	s.Require().Error(err)
}

func (s *KeycapKitToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *k.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.KitToAPI(context.Background(), k, false, repository.ProfilePreferences{Currency: "EUR"})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Nil(out.Purchase.Currency)
	s.Equal(k.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(k.Purchase.OrderStatus, out.Purchase.OrderStatus)
}

func (s *KeycapKitToAPISuite) TestOwner_IncludesPrice() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *k.ImagePath).Return("https://example.com/signed-get", nil)
	kr := KeycapSet{Images: images}
	out, err := kr.KitToAPI(context.Background(), k, true, repository.ProfilePreferences{Currency: "EUR"})
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(k.Purchase.Price, out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Currency)
	s.Equal("EUR", *out.Purchase.Currency)
}

func (s *KeycapKitToAPISuite) TestSignGetFails_ReturnsError() {
	k := repository.KeycapKit{KitID: "kit1", Name: "Base", ImagePath: imageKeyPtr("keycap-sets/alice/ks1/kits/kit1/image/up1")}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, *k.ImagePath).Return("", errors.New("s3: access denied"))

	kr := KeycapSet{Images: images}
	_, err := kr.KitToAPI(context.Background(), k, true, repository.ProfilePreferences{Currency: "EUR"})

	s.Require().Error(err)
}

func fullAPIKeycapKitInput() api.KeycapKitInput {
	return api.KeycapKitInput{
		Name: "Base",
		Purchase: &api.PurchaseInput{
			Vendor:                strPtr("CannonKeys"),
			Price:                 floatPtr(120.00),
			OrderStatus:           strPtr("delivered"),
			EstimatedDeliveryDate: &openapi_types.Date{Time: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)},
		},
	}
}

type KeycapKitToRepoSuite struct {
	suite.Suite
}

func TestKeycapKitToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeycapKitToRepoSuite))
}

func (s *KeycapKitToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIKeycapKitInput()
	out := KeycapSet{}.KitToRepo(in)

	s.Equal(in.Name, out.Name)
	s.Equal(in.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(in.Purchase.Price, out.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Equal(strPtr("2026-03-31"), out.Purchase.EstimatedDeliveryDate)
	s.Empty(out.KitID)
}

func (s *KeycapKitToRepoSuite) TestPurchaseNil_MapsToZeroValue() {
	in := api.KeycapKitInput{Name: "Base"}

	out := KeycapSet{}.KitToRepo(in)

	s.Equal(repository.KeycapKitPurchase{}, out.Purchase)
}
