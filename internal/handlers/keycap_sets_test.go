package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/ownerprefs"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type ListKeycapSetsSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestListKeycapSetsSuite(t *testing.T) {
	suite.Run(t, new(ListKeycapSetsSuite))
}

func (s *ListKeycapSetsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, "alice", mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
	s.handler = ListKeycapSets(s.mockRepo, s.mockBuilds, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_BaseOnCurrentBuild_NotAvailable() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{
			{ID: "ks1", Visibility: repository.VisibilityPublic, Kits: map[string]repository.KeycapKit{
				"k1": {KitID: "k1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderStatus: new("Delivered")}},
			}},
			{ID: "ks2", Visibility: repository.VisibilityPublic, Kits: map[string]repository.KeycapKit{
				"k1": {KitID: "k1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderStatus: new("Delivered")}},
			}},
		}, "", nil)
	s.builds = []repository.Build{{Keyboard: "kb1", Visibility: repository.VisibilityPrivate, KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks1", Kit: "k1"}}}}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(*got.Items, 2)
	s.False((*got.Items)[0].Available)
	s.True((*got.Items)[1].Available)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_BuildsError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{ID: "ks1"}}, "", nil)
	s.buildsErr = errors.New("query failed")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ListKeycapSetsSuite) newRequest(ctx context.Context, query string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keycap-sets?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_Owner_RequestsAllVisibilities() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.KeycapSet{{ID: "ks1", Brand: "GMK", Name: "Laser", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	visibility := api.VisibilityPrivate
	s.Equal(&[]api.KeycapSet{{Id: "ks1", Brand: "GMK", Name: "Laser", Visibility: &visibility}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.KeycapSet{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.KeycapSet{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_OwnerShowPriceToMeTrue_IncludesTotalCost() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 120.00

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{
			ID:   "ks1",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Require().NotNil(item.TotalCost)
	s.InDelta(price, *item.TotalCost, 0.0001)
	s.Require().NotNil(item.Currency)
	s.Equal("EUR", *item.Currency)
	s.Require().NotNil(item.Kits)
	s.Require().NotNil((*item.Kits)[0].Purchase)
	s.Require().NotNil((*item.Kits)[0].Purchase.Price)
	s.InDelta(price, *(*item.Kits)[0].Purchase.Price, 0.0001)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_OwnerShowPriceToMeFalse_OmitsTotalCost() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 120.00

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{
			ID:   "ks1",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Nil(item.TotalCost)
	s.Nil(item.Currency)
	s.Require().NotNil(item.Kits)
	s.Nil((*item.Kits)[0].Purchase)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_NonOwnerShowPriceToOthersFalse_OmitsTotalCost() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 120.00

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{
			ID:   "ks1",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Nil(item.TotalCost)
	s.Nil(item.Currency)
	s.Require().NotNil(item.Kits)
	s.Nil((*item.Kits)[0].Purchase)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_NonOwnerShowPriceToOthersTrue_IncludesTotalCost() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 120.00

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{
			ID:   "ks1",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Require().NotNil(item.TotalCost)
	s.InDelta(price, *item.TotalCost, 0.0001)
	s.Require().NotNil(item.Currency)
	s.Equal("EUR", *item.Currency)
	s.Require().NotNil(item.Kits)
	s.Require().NotNil((*item.Kits)[0].Purchase)
	s.Require().NotNil((*item.Kits)[0].Purchase.Price)
	s.InDelta(price, *(*item.Kits)[0].Purchase.Price, 0.0001)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.KeycapSet{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{}, "next-page-token", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_PrimaryKitWithImage_ReturnsPrimaryKitIDAndKitImage() {
	imagePath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	kitID := "kit1"

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.KeycapSet{{
			ID:           "ks1",
			Brand:        "GMK",
			Name:         "Laser",
			PrimaryKitID: &kitID,
			Kits:         map[string]repository.KeycapKit{kitID: {KitID: kitID, ImagePath: &imagePath}},
		}}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	s.mockImages.EXPECT().SignGet(mock.Anything, imagePath).Return("https://example.com/signed-get", nil)
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)

	var got api.KeycapSetListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	item := (*got.Items)[0]
	s.Require().NotNil(item.PrimaryKitId)
	s.Equal(kitID, *item.PrimaryKitId)
	s.Require().NotNil(item.Kits)
	s.Require().NotNil((*item.Kits)[0].Image)
	s.Equal("https://example.com/signed-get", *(*item.Kits)[0].Image.Url)
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListKeycapSetsSuite) TestListKeycapSets_InvalidCursor_Returns400() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20&cursor=stale"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetKeycapSetSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestGetKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(GetKeycapSetSuite))
}

func (s *GetKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, "alice", mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
	s.handler = GetKeycapSet(s.mockRepo, s.mockBuilds, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_DeliveredBaseNotOnBuild_Available() {
	ks := repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic, Kits: map[string]repository.KeycapKit{
		"k1": {KitID: "k1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderStatus: new("Delivered")}},
	}}
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(&ks, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.True(got.Available)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_BuildsError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic}, nil)
	s.buildsErr = errors.New("query failed")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *GetKeycapSetSuite) newRequest(ctx context.Context) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keycap-sets/ks1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_Owner_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.prefs = repository.DefaultProfilePreferences()

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Brand: "GMK", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("ks1", got.ID)
	s.Equal("GMK", got.Brand)
	s.Equal(repository.VisibilityPrivate, got.Visibility)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_KitWithImagePath_IncludesSignedURL() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.prefs = repository.DefaultProfilePreferences()
	imagePath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID:         "ks1",
			Visibility: repository.VisibilityPrivate,
			Kits:       map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base", ImagePath: &imagePath}},
		}, nil)
	s.mockImages.EXPECT().
		SignGet(mock.Anything, imagePath).
		Return("https://example.com/signed-get", nil)
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Kits)
	s.Require().Len(*got.Kits, 1)
	s.Require().NotNil((*got.Kits)[0].Image)
	s.Equal("https://example.com/signed-get", *(*got.Kits)[0].Image.Url)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPublicKeycapSet_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_AnonymousReadingPrivateKeycapSet_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingAuthenticatedKeycapSet_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityAuthenticated}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Nil(got.Visibility)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_NonOwnerShowPriceToOthersTrue_IncludesKitPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 120.00

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID: "ks1", Visibility: repository.VisibilityPublic,
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Kits)
	s.Require().Len(*got.Kits, 1)
	s.Require().NotNil((*got.Kits)[0].Purchase)
	s.Require().NotNil((*got.Kits)[0].Purchase.Price)
	s.InDelta(price, *(*got.Kits)[0].Purchase.Price, 0.0001)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_NonOwnerShowPriceToOthersFalse_OmitsKitPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 120.00

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID: "ks1", Visibility: repository.VisibilityPublic,
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Kits)
	s.Require().Len(*got.Kits, 1)
	s.Require().NotNil((*got.Kits)[0].Purchase)
	s.Nil((*got.Kits)[0].Purchase.Price)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_Owner_AlwaysIncludesKitPriceAndCurrency() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 120.00

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID: "ks1", Visibility: repository.VisibilityPrivate,
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Kits)
	s.Require().Len(*got.Kits, 1)
	s.Require().NotNil((*got.Kits)[0].Purchase)
	s.Require().NotNil((*got.Kits)[0].Purchase.Price)
	s.InDelta(price, *(*got.Kits)[0].Purchase.Price, 0.0001)
	s.Require().NotNil((*got.Kits)[0].Purchase.Currency)
	s.Equal("EUR", *(*got.Kits)[0].Purchase.Currency)
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_OtherUserReadingPrivateKeycapSet_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeycapSetSuite) TestGetKeycapSet_MalformedKitPurchaseDate_Returns500NotPanic() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.prefs = repository.DefaultProfilePreferences()
	malformedDate := "not-a-date"
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			ID:         "ks1",
			Visibility: repository.VisibilityPrivate,
			Kits:       map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{OrderDate: &malformedDate}}},
		}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeycapSetSuite struct {
	suite.Suite

	prefs             repository.ProfilePreferences
	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockImages        *mocks.MockKeycapKitImageStore
	handler           http.HandlerFunc
}

func TestCreateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(CreateKeycapSetSuite))
}

func (s *CreateKeycapSetSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = CreateKeycapSet(s.mockKeycapSetRepo, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *CreateKeycapSetSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateKeycapSetSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Succeeds() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.ID != "" && ks.Brand == "GMK" && ks.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "generated-id", Brand: "GMK", Name: "Laser"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.Id)
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Visibility_Preserved() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"profile", `{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved"}`},
		{"material", `{"brand":"GMK","name":"Laser","visibility":"private","material":"NotApproved"}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

			var got struct {
				InvalidParams []problem.InvalidParam `json:"invalid_params"`
			}
			s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
			s.Require().Len(got.InvalidParams, 1)
			s.Equal(tt.name, got.InvalidParams[0].Name)
		})
	}
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_MultipleInvalidFields_NamesAll() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved",`+
			`"material":"AlsoNotApproved"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	names := make([]string, len(got.InvalidParams))
	for i, p := range got.InvalidParams {
		names[i] = p.Name
	}
	s.Contains(names, "profile")
	s.Contains(names, "material")
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_AlreadyExists_Returns409() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_RepositoryError_Returns500() {
	s.mockKeycapSetRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateKeycapSetSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	prefs             repository.ProfilePreferences
	mockKeycapSetRepo *mocks.MockKeycapSetRepository
	mockImages        *mocks.MockKeycapKitImageStore
	handler           http.HandlerFunc
}

func TestUpdateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(UpdateKeycapSetSuite))
}

func (s *UpdateKeycapSetSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockKeycapSetRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, "alice", mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
	s.handler = UpdateKeycapSet(s.mockKeycapSetRepo, s.mockBuilds, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *UpdateKeycapSetSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/keycap-sets/ks1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *UpdateKeycapSetSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_BaseOnCurrentBuild_NotAvailable() {
	ks := repository.KeycapSet{ID: "ks1", Visibility: repository.VisibilityPublic, Kits: map[string]repository.KeycapKit{
		"k1": {KitID: "k1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderStatus: new("Delivered")}},
	}}
	s.mockKeycapSetRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(&ks, nil)
	s.builds = []repository.Build{{Keyboard: "kb1", Visibility: repository.VisibilityPrivate, KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks1", Kit: "k1"}}}}

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.False(got.Available)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_BuildsError_Returns500WithoutUpdating() {
	s.buildsErr = errors.New("query failed")

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Succeeds() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.ID == "ks1" && ks.Brand == "GMK" && ks.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Laser"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("ks1", got.Id)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Visibility_Preserved() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(ks repository.KeycapSet) bool {
			return ks.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"profile", `{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved"}`},
		{"material", `{"brand":"GMK","name":"Laser","visibility":"private","material":"NotApproved"}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

			var got struct {
				InvalidParams []problem.InvalidParam `json:"invalid_params"`
			}
			s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
			s.Require().Len(got.InvalidParams, 1)
			s.Equal(tt.name, got.InvalidParams[0].Name)
		})
	}
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_MultipleInvalidFields_NamesAll() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"GMK","name":"Laser","visibility":"private","profile":"NotApproved",`+
			`"material":"AlsoNotApproved"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	names := make([]string, len(got.InvalidParams))
	for i, p := range got.InvalidParams {
		names[i] = p.Name
	}
	s.Contains(names, "profile")
	s.Contains(names, "material")
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_NotFound_Returns404() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_RepositoryError_Returns500() {
	s.mockKeycapSetRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapSetSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
	handler      http.HandlerFunc
}

func TestDeleteKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapSetSuite))
}

func (s *DeleteKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
}

func (s *DeleteKeycapSetSuite) newRequest(ctx context.Context, onDelete string) *http.Request {
	target := "/users/alice/keycap-sets/ks1"
	if onDelete != "" {
		target += "?on_delete=" + onDelete
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *DeleteKeycapSetSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Owner_Succeeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1"}, nil)
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_KitsWithImages_DeletesEachFromS3BeforeDB() {
	key1 := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	key2 := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit2/image/up1")
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &key1},
			"kit2": {KitID: "kit2", ImagePath: &key2}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key1).
		Return(nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key2).
		Return(nil)
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_KitImageDeleteFails_Returns500_DoesNotDeleteSet() {
	key1 := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &key1}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key1).
		Return(errors.New("s3 unavailable"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_RepositoryNotFound_StillReturns204() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_RepositoryError_Returns500() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1"}, nil)
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Block_Referenced_Returns409WithBlockingBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return([]string{"build-1", "build-2"}, nil)

	req := s.newRequest(s.ownerCtx(), "block")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body struct {
		BlockingBuildIDs []string `json:"blocking_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.ElementsMatch([]string{"build-1", "build-2"}, body.BlockingBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_Cascade_Referenced_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, "alice", "ks1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1"}, nil)
	s.mockRepo.EXPECT().
		Delete(mock.Anything, "ks1").
		Return(nil)

	req := s.newRequest(s.ownerCtx(), "cascade")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var body struct {
		DeletedBuildIDs []string `json:"deleted_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.Equal([]string{"build-1"}, body.DeletedBuildIDs)
}

func (s *DeleteKeycapSetSuite) TestDeleteKeycapSet_InvalidOnDelete_Returns400() {
	req := s.newRequest(s.ownerCtx(), "bogus")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

type CreateKeycapKitSuite struct {
	suite.Suite

	prefs      repository.ProfilePreferences
	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestCreateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(CreateKeycapKitSuite))
}

func (s *CreateKeycapKitSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = CreateKeycapKit(s.mockRepo, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *CreateKeycapKitSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets/ks1/kits", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	return req
}

func (s *CreateKeycapKitSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_Succeeds() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.MatchedBy(func(k repository.KeycapKit) bool {
			return k.KitID != "" && k.Name == "Base"
		}), mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kit1", got.KitId)
	s.Equal("Base", got.Name)
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_PrimaryTrue_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && *primary
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base","primary":true}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_PrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"purchase.vendor", `{"name":"Base","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"name":"Base","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

			var got struct {
				InvalidParams []problem.InvalidParam `json:"invalid_params"`
			}
			s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
			s.Require().Len(got.InvalidParams, 1)
			s.Equal(tt.name, got.InvalidParams[0].Name)
		})
	}
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_ParentSetNotFound_Returns404() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"name":"Base"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateKeycapKitSuite struct {
	suite.Suite

	prefs      repository.ProfilePreferences
	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestUpdateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(UpdateKeycapKitSuite))
}

func (s *UpdateKeycapKitSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = UpdateKeycapKit(s.mockRepo, repoapi.KeycapSet{Images: s.mockImages})
}

func (s *UpdateKeycapKitSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/keycap-sets/ks1/kits/kit1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *UpdateKeycapKitSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_Succeeds() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.MatchedBy(func(k repository.KeycapKit) bool {
			return k.KitID == "kit1" && k.Name == "Extension"
		}), mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Extension"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kit1", got.KitId)
	s.Equal("Extension", got.Name)
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_PrimaryFalse_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && !*primary
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Extension"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension","primary":false}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_PrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Extension"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"purchase.vendor", `{"name":"Extension","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"name":"Extension","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

			var got struct {
				InvalidParams []problem.InvalidParam `json:"invalid_params"`
			}
			s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
			s.Require().Len(got.InvalidParams, 1)
			s.Equal(tt.name, got.InvalidParams[0].Name)
		})
	}
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"name":"Extension"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapKitSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
	handler      http.HandlerFunc
}

func TestDeleteKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapKitSuite))
}

func (s *DeleteKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
}

func (s *DeleteKeycapKitSuite) newRequest(ctx context.Context, onDelete string) *http.Request {
	target := "/users/alice/keycap-sets/ks1/kits/kit1"
	if onDelete != "" {
		target += "?on_delete=" + onDelete
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *DeleteKeycapKitSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Succeeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1"}}}, nil)
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil)

	req := s.newRequest(s.ownerCtx(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_KitHadImage_DeletesFromS3BeforeDB() {
	key := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key).
		Return(nil)
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil)

	req := s.newRequest(s.ownerCtx(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_ImageDeleteFails_Returns500_DoesNotDeleteKit() {
	key := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, key).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_ParentSetNotFound_Returns404() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_RepositoryError_Returns500() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1"}}}, nil)
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), "")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Block_Referenced_Returns409WithBlockingBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return([]string{"build-1", "build-2"}, nil)

	req := s.newRequest(s.ownerCtx(), "block")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body struct {
		BlockingBuildIDs []string `json:"blocking_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.ElementsMatch([]string{"build-1", "build-2"}, body.BlockingBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_Cascade_Referenced_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, "alice", "ks1", "kit1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1"}}}, nil)
	s.mockRepo.EXPECT().
		DeleteKit(mock.Anything, "ks1", "kit1").
		Return(nil)

	req := s.newRequest(s.ownerCtx(), "cascade")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var body struct {
		DeletedBuildIDs []string `json:"deleted_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.Equal([]string{"build-1"}, body.DeletedBuildIDs)
}

func (s *DeleteKeycapKitSuite) TestDeleteKeycapKit_InvalidOnDelete_Returns400() {
	req := s.newRequest(s.ownerCtx(), "bogus")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

type SetKeycapKitImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockKeycapSetRepository
	mockImages  *mocks.MockKeycapKitImageStore
	mockUploads *mocks.MockUploadCounter
	handler     http.HandlerFunc
}

func TestSetKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(SetKeycapKitImageSuite))
}

func (s *SetKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
	s.handler = SetKeycapKitImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
}

func (s *SetKeycapKitImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keycap-sets/ks1/kits/kit1/image", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *SetKeycapKitImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

const previousKeycapKitImageTestKey = repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/old-upload")

func newKeycapKitImageKey() any {
	return mock.MatchedBy(func(k repository.KeycapKitImageKey) bool {
		return strings.HasPrefix(string(k), "keycap-sets/alice/ks1/kits/kit1/image/") && k != previousKeycapKitImageTestKey
	})
}

func setWithKit1(imagePath *repository.KeycapKitImageKey) *repository.KeycapSet {
	return &repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {ImagePath: imagePath}}}
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_Succeeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(setWithKit1(nil), nil)
	var presigned repository.KeycapKitImageKey
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKeycapKitImageKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.KeycapKitImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/presigned-put", nil).
		NotBefore(reserved)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", newKeycapKitImageKey()).
		Run(func(_ context.Context, _, _ string, key repository.KeycapKitImageKey) { s.Equal(presigned, key) }).
		Return(nil)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got struct {
		UploadURL string `json:"upload_url"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("https://example.com/presigned-put", got.UploadURL)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_ReplacingImage_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousKeycapKitImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(setWithKit1(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKeycapKitImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	deleted := s.mockImages.EXPECT().
		Delete(mock.Anything, previousKeycapKitImageTestKey).
		Return(nil).
		Call
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", newKeycapKitImageKey()).
		Return(nil).
		NotBefore(deleted)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_UnapprovedContentType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"content_type":"application/pdf"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("content_type", got.InvalidParams[0].Name)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_SetNotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_KitNotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_PresignError_Returns500() {
	expectUploadReserved(s.mockUploads)
	previous := previousKeycapKitImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(setWithKit1(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKeycapKitImageKey(), "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_DeletePreviousError_Returns500() {
	expectUploadReserved(s.mockUploads)
	previous := previousKeycapKitImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(setWithKit1(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKeycapKitImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, previousKeycapKitImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_RepositoryError_Returns500() {
	expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(setWithKit1(nil), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKeycapKitImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks1", "kit1", newKeycapKitImageKey()).
		Return(errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
	handler    http.HandlerFunc
}

func TestDeleteKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeycapKitImageSuite))
}

func (s *DeleteKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.handler = DeleteKeycapKitImage(s.mockRepo, s.mockImages)
}

func (s *DeleteKeycapKitImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/keycap-sets/ks1/kits/kit1/image", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keycapSetId", "ks1")
	req.SetPathValue("kitId", "kit1")
	return req
}

func (s *DeleteKeycapKitImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

var deleteKeycapKitImageTestKey = repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &deleteKeycapKitImageTestKey}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteKeycapKitImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(&deleteKeycapKitImageTestKey, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_ClearKitImagePathNotFound_Returns204() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &deleteKeycapKitImageTestKey}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteKeycapKitImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearKitImagePath(mock.Anything, "ks1", "kit1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1"}}}, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_KitNotInSet_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1"}, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeycapKitImageSuite) TestDeleteKeycapKitImage_S3DeleteError_Returns500_DoesNotClearDBRecord() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", ImagePath: &deleteKeycapKitImageTestKey}}}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteKeycapKitImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeycapSetSuite) TestCreateKeycapSet_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeycapSetRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`))

	s.Equal(http.StatusCreated, rec.Code)
	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil((*got.Kits)[0].Purchase.Currency)
	s.Equal("EUR", *(*got.Kits)[0].Purchase.Currency)
}

func (s *UpdateKeycapSetSuite) TestUpdateKeycapSet_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeycapSetRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Purchase: repository.KeycapKitPurchase{Price: &price}}}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"GMK","name":"Laser","visibility":"private"}`))

	s.Equal(http.StatusOK, rec.Code)
	var got api.KeycapSet
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil((*got.Kits)[0].Purchase.Currency)
	s.Equal("EUR", *(*got.Kits)[0].Purchase.Currency)
}

func (s *CreateKeycapKitSuite) TestCreateKeycapKit_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockRepo.EXPECT().AddKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"name":"Base"}`))

	s.Equal(http.StatusCreated, rec.Code)
	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase.Currency)
	s.Equal("EUR", *got.Purchase.Currency)
}

func (s *UpdateKeycapKitSuite) TestUpdateKeycapKit_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockRepo.EXPECT().UpdateKit(mock.Anything, "ks1", mock.Anything, mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"name":"Base"}`))

	s.Equal(http.StatusOK, rec.Code)
	var got api.KeycapKit
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase.Currency)
	s.Equal("EUR", *got.Purchase.Currency)
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_UploadLimitReached_Returns429() {
	previous := previousKeycapKitImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(setWithKit1(&previous), nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusTooManyRequests, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	s.Contains(rec.Body.String(), "image uploads aren't available at the moment")
}

func (s *SetKeycapKitImageSuite) TestSetKeycapKitImage_ReserveError_Returns500() {
	previous := previousKeycapKitImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(setWithKit1(&previous), nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}
