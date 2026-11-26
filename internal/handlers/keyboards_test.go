package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type ListKeyboardsSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeyboardRepository
	mockImages *mocks.MockKeyboardImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestListKeyboardsSuite(t *testing.T) {
	suite.Run(t, new(ListKeyboardsSuite))
}

func (s *ListKeyboardsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.handler = ListKeyboards(s.mockRepo, repoapi.Keyboard{Images: s.mockImages})
}

func (s *ListKeyboardsSuite) newRequest(ctx context.Context, query string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keyboards?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListKeyboardsSuite) TestListKeyboards_Owner_RequestsAllVisibilities() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.Keyboard{
			{ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate},
		}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	visibility := api.VisibilityPrivate
	s.Equal(&[]api.Keyboard{{Id: "kb1", Brand: "Keychron", Name: "Q1", Visibility: &visibility}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListKeyboardsSuite) TestListKeyboards_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.Keyboard{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.Keyboard{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_OwnerShowPriceToMeTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 199.99

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	purchase := (*got.Items)[0].Purchase
	s.Require().NotNil(purchase)
	s.Require().NotNil(purchase.Price)
	s.InDelta(price, *purchase.Price, 0.0001)
	s.Require().NotNil(purchase.Currency)
	s.Equal("EUR", *purchase.Currency)
}

func (s *ListKeyboardsSuite) TestListKeyboards_OwnerShowPriceToMeFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 199.99

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].Purchase)
}

func (s *ListKeyboardsSuite) TestListKeyboards_OwnerShowPriceToMeFalse_KeepsRestOfPurchase() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 199.99
	vendor := "Amazon"

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price, Vendor: &vendor}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	purchase := (*got.Items)[0].Purchase
	s.Require().NotNil(purchase)
	s.Nil(purchase.Price)
	s.Nil(purchase.Currency)
	s.Require().NotNil(purchase.Vendor)
	s.Equal(vendor, *purchase.Vendor)
}

func (s *ListKeyboardsSuite) TestListKeyboards_NonOwnerShowPriceToOthersFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 199.99

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].Purchase)
}

func (s *ListKeyboardsSuite) TestListKeyboards_NonOwnerShowPriceToOthersTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 199.99

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	purchase := (*got.Items)[0].Purchase
	s.Require().NotNil(purchase)
	s.Require().NotNil(purchase.Price)
	s.InDelta(price, *purchase.Price, 0.0001)
	s.Require().NotNil(purchase.Currency)
	s.Equal("EUR", *purchase.Currency)
}

func (s *ListKeyboardsSuite) TestListKeyboards_ReturnsEveryImage() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{{UserID: "alice", ID: "kb1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"},
			{ImageID: "img2", Path: "keyboards/alice/kb1/images/img2"},
		})}}, "", nil)
	s.mockImages.EXPECT().SignGetKeyboardImage(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, key repository.KeyboardImageKey) (string, error) {
			return "https://example.com/" + string(key), nil
		})
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Require().NotNil((*got.Items)[0].Images)
	s.Len(*(*got.Items)[0].Images, 2)
}

func (s *ListKeyboardsSuite) TestListKeyboards_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Keyboard{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListKeyboardsSuite) TestListKeyboards_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Keyboard{}, "next-page-token", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	var got api.KeyboardListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListKeyboardsSuite) TestListKeyboards_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListKeyboardsSuite) TestListKeyboards_InvalidCursor_Returns400() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20&cursor=stale"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetKeyboardSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeyboardRepository
	mockImages *mocks.MockKeyboardImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestGetKeyboardSuite(t *testing.T) {
	suite.Run(t, new(GetKeyboardSuite))
}

func (s *GetKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.handler = GetKeyboard(s.mockRepo, repoapi.Keyboard{Images: s.mockImages})
}

func (s *GetKeyboardSuite) newRequest(ctx context.Context) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/keyboards/kb1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keyboardId", "kb1")
	return req
}

func (s *GetKeyboardSuite) TestGetKeyboard_Owner_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.prefs = repository.DefaultProfilePreferences()

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Brand: "Keychron", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kb1", got.ID)
	s.Equal("Keychron", got.Brand)
	s.Equal(repository.VisibilityPrivate, got.Visibility)
}

func (s *GetKeyboardSuite) TestGetKeyboard_AnonymousReadingPublicKeyboard_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPublic}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetKeyboardSuite) TestGetKeyboard_AnonymousReadingPrivateKeyboard_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_OtherUserReadingAuthenticatedKeyboard_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityAuthenticated}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)

	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Nil(got.Visibility)
}

func (s *GetKeyboardSuite) TestGetKeyboard_NonOwnerShowPriceToOthersTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 199.99

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			ID: "kb1", Visibility: repository.VisibilityPublic,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase)
	s.Require().NotNil(got.Purchase.Price)
	s.InDelta(price, *got.Purchase.Price, 0.0001)
}

func (s *GetKeyboardSuite) TestGetKeyboard_NonOwnerShowPriceToOthersFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 199.99

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			ID: "kb1", Visibility: repository.VisibilityPublic,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase)
	s.Nil(got.Purchase.Price)
}

func (s *GetKeyboardSuite) TestGetKeyboard_Owner_AlwaysIncludesPriceAndCurrency() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 199.99

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			ID: "kb1", Visibility: repository.VisibilityPrivate,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase)
	s.Require().NotNil(got.Purchase.Price)
	s.InDelta(price, *got.Purchase.Price, 0.0001)
	s.Require().NotNil(got.Purchase.Currency)
	s.Equal("EUR", *got.Purchase.Currency)
}

func (s *GetKeyboardSuite) TestGetKeyboard_OtherUserReadingPrivateKeyboard_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetKeyboardSuite) TestGetKeyboard_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			ID:         "kb1",
			Visibility: repository.VisibilityPublic,
			Purchase:   repository.KeyboardPurchase{OrderDate: &malformedDate},
		}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateKeyboardSuite struct {
	suite.Suite

	prefs            repository.ProfilePreferences
	mockKeyboardRepo *mocks.MockKeyboardRepository
	mockImages       *mocks.MockKeyboardImageStore
	handler          http.HandlerFunc
}

func TestCreateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(CreateKeyboardSuite))
}

func (s *CreateKeyboardSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.handler = CreateKeyboard(s.mockKeyboardRepo, repoapi.Keyboard{Images: s.mockImages})
}

func (s *CreateKeyboardSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keyboards", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateKeyboardSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.ID != "" && kb.Brand == "Keychron" && kb.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id", Brand: "Keychron", Name: "Q1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.ID)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Visibility_Preserved() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"size", `{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved"}`},
		{"design.top_case.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"top_case":{"material":"NotApproved"}}}`},
		{"design.bottom_case.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"bottom_case":{"material":"NotApproved"}}}`},
		{"design.weight.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"weight":{"material":"NotApproved"}}}`},
		{"purchase.vendor", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
		})
	}
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutValidForSize_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"60%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutInvalidForSize_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"40%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("layout", got.InvalidParams[0].Name)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_DeliveryDateBeforeOrderDate_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"order_date":"2026-03-01","delivery_date":"2026-02-28"}}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal([]problem.InvalidParam{{Name: "purchase.delivery_date", Reason: "can't be before order_date"}}, got.InvalidParams)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_UnrecognizedLayout_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"NotALayout"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_LayoutWithoutSize_SkipsSizeCheck() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_MultipleInvalidFields_NamesAll() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved",`+
			`"design":{"top_case":{"material":"AlsoNotApproved"}}}`)
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
	s.Contains(names, "size")
	s.Contains(names, "design.top_case.material")
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_InvalidSize_DoesNotCascadeIntoLayoutError() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved","layout":"WK"}`)
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
	s.Equal([]string{"size"}, names)
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_AlreadyExists_Returns409() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_RepositoryError_Returns500() {
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockKeyboardRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{
			ID:         "kb1",
			Visibility: repository.VisibilityPrivate,
			Purchase:   repository.KeyboardPurchase{OrderDate: &malformedDate},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateKeyboardSuite struct {
	suite.Suite

	prefs            repository.ProfilePreferences
	mockKeyboardRepo *mocks.MockKeyboardRepository
	mockImages       *mocks.MockKeyboardImageStore
	handler          http.HandlerFunc
}

func TestUpdateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(UpdateKeyboardSuite))
}

func (s *UpdateKeyboardSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.handler = UpdateKeyboard(s.mockKeyboardRepo, repoapi.Keyboard{Images: s.mockImages})
}

func (s *UpdateKeyboardSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/keyboards/kb1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keyboardId", "kb1")
	return req
}

func (s *UpdateKeyboardSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.ID == "kb1" && kb.Brand == "Keychron" && kb.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("kb1", got.ID)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_Visibility_Preserved() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(kb repository.Keyboard) bool {
			return kb.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"size", `{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved"}`},
		{"design.top_case.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"top_case":{"material":"NotApproved"}}}`},
		{"design.bottom_case.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"bottom_case":{"material":"NotApproved"}}}`},
		{"design.weight.material", `{"brand":"Keychron","name":"Q1","visibility":"private","design":{"weight":{"material":"NotApproved"}}}`},
		{"purchase.vendor", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"vendor":"NotApproved"}}`},
		{"purchase.order_status", `{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"order_status":"NotApproved"}}`},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()

			req := s.newRequest(s.ownerCtx(), tt.body)
			rec := httptest.NewRecorder()
			s.handler(rec, req)

			s.Equal(http.StatusBadRequest, rec.Code)
			s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
		})
	}
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_LayoutValidForSize_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"60%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_LayoutInvalidForSize_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"40%","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("layout", got.InvalidParams[0].Name)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_UnrecognizedLayout_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"NotALayout"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_LayoutWithoutSize_SkipsSizeCheck() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil)

	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"WK"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_MultipleInvalidFields_NamesAll() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved",`+
			`"design":{"top_case":{"material":"AlsoNotApproved"}}}`)
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
	s.Contains(names, "size")
	s.Contains(names, "design.top_case.material")
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_InvalidSize_DoesNotCascadeIntoLayoutError() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved","layout":"WK"}`)
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
	s.Equal([]string{"size"}, names)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_NotFound_Returns404() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_RepositoryError_Returns500() {
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockKeyboardRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{
			ID:         "kb1",
			Visibility: repository.VisibilityPrivate,
			Purchase:   repository.KeyboardPurchase{OrderDate: &malformedDate},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeyboardSuite struct {
	suite.Suite

	mockKeyboards      *mocks.MockKeyboardRepository
	mockBuilds         *mocks.MockBuildRepository
	mockBuildImages    *mocks.MockBuildImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
	handler            http.HandlerFunc
}

func TestDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeyboardSuite))
}

func (s *DeleteKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
	s.handler = DeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
}

func (s *DeleteKeyboardSuite) newRequest(ctx context.Context, onDelete string) *http.Request {
	target := "/users/alice/keyboards/kb1"
	if onDelete != "" {
		target += "?on_delete=" + onDelete
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keyboardId", "kb1")
	return req
}

func (s *DeleteKeyboardSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_Owner_DefaultOnDelete_NoReferences_Returns204() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_Owner_Block_Referenced_Returns409WithBlockingBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, "alice", "kb1").
		Return([]string{"build-1", "build-2"}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "block"))

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body struct {
		BlockingBuildIDs []string `json:"blocking_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.ElementsMatch([]string{"build-1", "build-2"}, body.BlockingBuildIDs)
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_Owner_Cascade_Referenced_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, "alice", "kb1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "cascade"))

	s.Equal(http.StatusOK, rec.Code)

	var body struct {
		DeletedBuildIDs []string `json:"deleted_build_ids"`
	}
	s.Require().NoError(json.NewDecoder(rec.Body).Decode(&body))
	s.Equal([]string{"build-1"}, body.DeletedBuildIDs)
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_Owner_InvalidOnDelete_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "bogus"))

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_RepositoryError_Returns500() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb1").
		Return(errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardSuite) TestDeleteKeyboard_ImageDeleteFails_Returns500_DoesNotDeleteKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, "alice", "kb1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"},
		})}, nil)
	s.mockKeyboardImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")).
		Return(errors.New("s3 unavailable"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type AddKeyboardImageSuite struct {
	suite.Suite

	mockKeyboardRepo *mocks.MockKeyboardRepository
	mockImages       *mocks.MockKeyboardImageStore
	mockUploads      *mocks.MockUploadCounter
	handler          http.HandlerFunc
}

func TestAddKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(AddKeyboardImageSuite))
}

func (s *AddKeyboardImageSuite) SetupTest() {
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
	s.handler = AddKeyboardImage(s.mockKeyboardRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
}

func (s *AddKeyboardImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/keyboards/kb1/images", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keyboardId", "kb1")
	return req
}

func (s *AddKeyboardImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_Succeeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockKeyboardRepo.EXPECT().
		AddImage(mock.Anything, "kb1", mock.MatchedBy(func(img repository.KeyboardImage) bool {
			return img.ImageID != ""
		})).
		Return(nil)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil).
		NotBefore(reserved)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got struct {
		ImageID   string `json:"image_id"`
		UploadURL string `json:"upload_url"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.NotEmpty(got.ImageID)
	s.Equal("https://example.com/presigned-put", got.UploadURL)
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_UnapprovedContentType_Returns400() {
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

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_NotFound_Returns404() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockKeyboardRepo.EXPECT().
		AddImage(mock.Anything, "kb1", mock.Anything).
		Return(repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_ImageLimitReached_Returns409() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockKeyboardRepo.EXPECT().
		AddImage(mock.Anything, "kb1", mock.Anything).
		Return(fmt.Errorf("adding image: %w", repository.ErrImageLimitReached))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_PresignError_Returns500() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_RepositoryError_Returns500() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockKeyboardRepo.EXPECT().
		AddImage(mock.Anything, "kb1", mock.Anything).
		Return(errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteKeyboardImageSuite struct {
	suite.Suite

	mockKeyboardRepo *mocks.MockKeyboardRepository
	mockImages       *mocks.MockKeyboardImageStore
	handler          http.HandlerFunc
}

func TestDeleteKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteKeyboardImageSuite))
}

func (s *DeleteKeyboardImageSuite) SetupTest() {
	s.mockKeyboardRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.handler = DeleteKeyboardImage(s.mockKeyboardRepo, s.mockImages)
}

func (s *DeleteKeyboardImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/keyboards/kb1/images/img1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("keyboardId", "kb1")
	req.SetPathValue("imageId", "img1")
	return req
}

func (s *DeleteKeyboardImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

var deleteKeyboardImageTestKey = repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_Succeeds() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img1", Path: deleteKeyboardImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, deleteKeyboardImageTestKey).
		Return(nil)
	s.mockKeyboardRepo.EXPECT().
		DeleteImage(mock.Anything, "kb1", "img1").
		Return(&deleteKeyboardImageTestKey, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_DeleteImageNotFound_Returns204() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img1", Path: deleteKeyboardImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, deleteKeyboardImageTestKey).
		Return(nil)
	s.mockKeyboardRepo.EXPECT().
		DeleteImage(mock.Anything, "kb1", "img1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1"}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_NotFound_Returns404() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_RepositoryError_Returns500() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteKeyboardImageSuite) TestDeleteKeyboardImage_S3DeleteError_Returns500_DoesNotDeleteDBRecord() {
	s.mockKeyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{ID: "kb1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img1", Path: deleteKeyboardImageTestKey},
		})}, nil)
	s.mockImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, deleteKeyboardImageTestKey).
		Return(errors.New("s3: access denied"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateKeyboardSuite) TestCreateKeyboard_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeyboardRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`))

	s.Equal(http.StatusCreated, rec.Code)
	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase.Currency)
	s.Equal("EUR", *got.Purchase.Currency)
}

func (s *UpdateKeyboardSuite) TestUpdateKeyboard_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeyboardRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Purchase: repository.KeyboardPurchase{Price: &price}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"Keychron","name":"Q1","visibility":"private"}`))

	s.Equal(http.StatusOK, rec.Code)
	var got api.Keyboard
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchase.Currency)
	s.Equal("EUR", *got.Purchase.Currency)
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_UploadLimitReached_Returns429() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusTooManyRequests, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	s.Contains(rec.Body.String(), "image uploads aren't available at the moment")
}

func (s *AddKeyboardImageSuite) TestAddKeyboardImage_ReserveError_Returns500() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}
