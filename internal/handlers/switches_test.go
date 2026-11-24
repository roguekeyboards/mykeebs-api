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

type ListSwitchesSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestListSwitchesSuite(t *testing.T) {
	suite.Run(t, new(ListSwitchesSuite))
}

func (s *ListSwitchesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.handler = ListSwitches(s.mockRepo, repoapi.Switch{Images: s.mockImages})
}

func (s *ListSwitchesSuite) newRequest(ctx context.Context, query string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches?"+query, nil)
	req.SetPathValue("userId", "alice")
	return req
}

func (s *ListSwitchesSuite) TestListSwitches_Owner_RequestsAllVisibilities() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 3
		}), 20, "").
		Return([]repository.Switch{{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	visibility := api.VisibilityPrivate
	s.Equal(&[]api.Switch{{Id: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: &visibility}}, got.Items)
	s.Nil(got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_Anonymous_RequestsPublicOnly() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, 20, "").
		Return([]repository.Switch{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_OtherUser_RequestsPublicAndAuthenticated() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool {
			return len(vis) == 2
		}), 20, "").
		Return([]repository.Switch{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_OwnerShowPriceToMeTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 8.50

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{{ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Require().NotNil((*got.Items)[0].Purchases)
	s.Require().Len(*(*got.Items)[0].Purchases, 1)
	purchase := (*(*got.Items)[0].Purchases)[0]
	s.Require().NotNil(purchase.Price)
	s.InDelta(price, *purchase.Price, 0.0001)
	s.Require().NotNil(purchase.Currency)
	s.Equal("EUR", *purchase.Currency)
}

func (s *ListSwitchesSuite) TestListSwitches_OwnerShowPriceToMeFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 8.50

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{{ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].Purchases)
}

func (s *ListSwitchesSuite) TestListSwitches_OwnerShowPriceToMeFalse_KeepsRestOfPurchase() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 8.50
	vendor := "CannonKeys"

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{{ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price, Vendor: &vendor}}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Require().NotNil((*got.Items)[0].Purchases)
	s.Require().Len(*(*got.Items)[0].Purchases, 1)
	purchase := (*(*got.Items)[0].Purchases)[0]
	s.Nil(purchase.Price)
	s.Nil(purchase.Currency)
	s.Require().NotNil(purchase.Vendor)
	s.Equal(vendor, *purchase.Vendor)
}

func (s *ListSwitchesSuite) TestListSwitches_NonOwnerShowPriceToOthersFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 8.50

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{{ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Nil((*got.Items)[0].Purchases)
}

func (s *ListSwitchesSuite) TestListSwitches_NonOwnerShowPriceToOthersTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 8.50

	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{{ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	s.Require().NotNil((*got.Items)[0].Purchases)
	s.Require().Len(*(*got.Items)[0].Purchases, 1)
	purchase := (*(*got.Items)[0].Purchases)[0]
	s.Require().NotNil(purchase.Price)
	s.InDelta(price, *purchase.Price, 0.0001)
	s.Require().NotNil(purchase.Currency)
	s.Equal("EUR", *purchase.Currency)
}

func (s *ListSwitchesSuite) TestListSwitches_PassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 5, "abc").
		Return([]repository.Switch{}, "", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListSwitchesSuite) TestListSwitches_ReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return([]repository.Switch{}, "next-page-token", nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	var got api.SwitchListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page-token", *got.NextCursor)
}

func (s *ListSwitchesSuite) TestListSwitches_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListSwitchesSuite) TestListSwitches_InvalidCursor_Returns400() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, 20, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "limit=20&cursor=stale"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetSwitchSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	prefs      repository.ProfilePreferences
	handler    http.HandlerFunc
}

func TestGetSwitchSuite(t *testing.T) {
	suite.Run(t, new(GetSwitchSuite))
}

func (s *GetSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.prefs = repository.ProfilePreferences{}
	s.handler = GetSwitch(s.mockRepo, repoapi.Switch{Images: s.mockImages})
}

func (s *GetSwitchSuite) newRequest(ctx context.Context) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/users/alice/switches/sw1", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *GetSwitchSuite) TestGetSwitch_Owner_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	s.prefs = repository.DefaultProfilePreferences()

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Brand: "Gateron", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("sw1", got.ID)
	s.Equal("Gateron", got.Brand)
	s.Equal(repository.VisibilityPrivate, got.Visibility)
}

func (s *GetSwitchSuite) TestGetSwitch_AnonymousReadingPublicSwitch_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPublic}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetSwitchSuite) TestGetSwitch_AnonymousReadingPrivateSwitch_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingAuthenticatedSwitch_Succeeds() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityAuthenticated}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusOK, rec.Code)

	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Nil(got.Visibility)
}

func (s *GetSwitchSuite) TestGetSwitch_NonOwnerShowPriceToOthersTrue_IncludesPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 8.50

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			ID: "sw1", Visibility: repository.VisibilityPublic,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: true}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchases)
	s.Require().Len(*got.Purchases, 1)
	s.Require().NotNil((*got.Purchases)[0].Price)
	s.InDelta(price, *(*got.Purchases)[0].Price, 0.0001)
}

func (s *GetSwitchSuite) TestGetSwitch_NonOwnerShowPriceToOthersFalse_OmitsPrice() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")
	price := 8.50

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			ID: "sw1", Visibility: repository.VisibilityPublic,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.prefs = repository.ProfilePreferences{ShowPriceToOthers: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchases)
	s.Require().Len(*got.Purchases, 1)
	s.Nil((*got.Purchases)[0].Price)
}

func (s *GetSwitchSuite) TestGetSwitch_Owner_AlwaysIncludesPriceAndCurrency() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	price := 8.50

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			ID: "sw1", Visibility: repository.VisibilityPrivate,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.prefs = repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchases)
	s.Require().Len(*got.Purchases, 1)
	s.Require().NotNil((*got.Purchases)[0].Price)
	s.InDelta(price, *(*got.Purchases)[0].Price, 0.0001)
	s.Require().NotNil((*got.Purchases)[0].Currency)
	s.Equal("EUR", *(*got.Purchases)[0].Currency)
}

func (s *GetSwitchSuite) TestGetSwitch_OtherUserReadingPrivateSwitch_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, errors.New("get item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetSwitchSuite) TestGetSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPublic,
			Purchases:  repository.BulkPurchases{{OrderDate: &malformedDate}},
		}, nil)
	s.prefs = repository.ProfilePreferences{}

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type CreateSwitchSuite struct {
	suite.Suite

	prefs          repository.ProfilePreferences
	mockSwitchRepo *mocks.MockSwitchRepository
	mockImages     *mocks.MockSwitchImageStore
	handler        http.HandlerFunc
}

func TestCreateSwitchSuite(t *testing.T) {
	suite.Run(t, new(CreateSwitchSuite))
}

func (s *CreateSwitchSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = CreateSwitch(s.mockSwitchRepo, repoapi.Switch{Images: s.mockImages})
}

func (s *CreateSwitchSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/switches", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	return req
}

func (s *CreateSwitchSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateSwitchSuite) TestCreateSwitch_Succeeds() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.ID != "" && sw.Brand == "Gateron" && sw.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Switch{UserID: "alice", ID: "generated-id", Brand: "Gateron", Name: "Yellow", Type: "Linear"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("generated-id", got.ID)
}

func (s *CreateSwitchSuite) TestCreateSwitch_Visibility_Preserved() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Switch{UserID: "alice", ID: "generated-id"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateSwitchSuite) TestCreateSwitch_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"type", `{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private"}`},
		{"material.top_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"top_housing":"NotApproved"}}`},
		{"material.bottom_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"bottom_housing":"NotApproved"}}`},
		{"material.stem", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`},
		{"spring.material", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","spring":{"material":"NotApproved"}}`},
		{"purchases[0].vendor", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchases":[{"vendor":"NotApproved"}]}`},
		{"purchases[0].order_status", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchases":[{"order_status":"NotApproved"}]}`},
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

func (s *CreateSwitchSuite) TestCreateSwitch_MultipleInvalidFields_NamesAll() {
	// type and material.top_housing are both invalid here - the response
	// must report both via invalid_params, not just the first one checked.
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private",`+
			`"material":{"top_housing":"AlsoNotApproved"}}`)
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
	s.Contains(names, "type")
	s.Contains(names, "material.top_housing")
}

func (s *CreateSwitchSuite) TestCreateSwitch_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_UnapprovedType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Bogus"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_UnapprovedLookupValue_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchases":[{"vendor":"NotARealVendor"}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_AlreadyExists_Returns409() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_RepositoryError_Returns500() {
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateSwitchSuite) TestCreateSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockSwitchRepo.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPrivate,
			Purchases:  repository.BulkPurchases{{OrderDate: &malformedDate}},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type UpdateSwitchSuite struct {
	suite.Suite

	prefs          repository.ProfilePreferences
	mockSwitchRepo *mocks.MockSwitchRepository
	mockImages     *mocks.MockSwitchImageStore
	handler        http.HandlerFunc
}

func TestUpdateSwitchSuite(t *testing.T) {
	suite.Run(t, new(UpdateSwitchSuite))
}

func (s *UpdateSwitchSuite) SetupTest() {
	s.prefs = repository.ProfilePreferences{Currency: "EUR"}
	s.mockSwitchRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = UpdateSwitch(s.mockSwitchRepo, repoapi.Switch{Images: s.mockImages})
}

func (s *UpdateSwitchSuite) newRequest(ctx context.Context, body string) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, s.prefs)
	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/users/alice/switches/sw1", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *UpdateSwitchSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Succeeds() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.ID == "sw1" && sw.Brand == "Gateron" && sw.Visibility == repository.VisibilityPrivate
		})).
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("sw1", got.ID)
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Visibility_Preserved() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(sw repository.Switch) bool {
			return sw.Visibility == repository.VisibilityPublic
		})).
		Return(&repository.Switch{UserID: "alice", ID: "sw1"}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"public"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_ValidatesOpenVocabularyFields() {
	tests := []struct {
		name string
		body string
	}{
		{"type", `{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private"}`},
		{"material.top_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"top_housing":"NotApproved"}}`},
		{"material.bottom_housing", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"bottom_housing":"NotApproved"}}`},
		{"material.stem", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`},
		{"spring.material", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","spring":{"material":"NotApproved"}}`},
		{"purchases[0].vendor", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchases":[{"vendor":"NotApproved"}]}`},
		{"purchases[0].order_status", `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","purchases":[{"order_status":"NotApproved"}]}`},
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

func (s *UpdateSwitchSuite) TestUpdateSwitch_MultipleInvalidFields_NamesAll() {
	// type and material.top_housing are both invalid here - the response
	// must report both via invalid_params, not just the first one checked.
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"NotApproved","visibility":"private",`+
			`"material":{"top_housing":"AlsoNotApproved"}}`)
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
	s.Contains(names, "type")
	s.Contains(names, "material.top_housing")
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_UnapprovedType_Returns400() {
	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Bogus"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_UnapprovedLookupValue_Returns400() {
	req := s.newRequest(s.ownerCtx(),
		`{"brand":"Gateron","name":"Yellow","type":"Linear","purchases":[{"vendor":"NotARealVendor"}]}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_NotFound_Returns404() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_RepositoryError_Returns500() {
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_MalformedStoredDate_Returns500NotPanic() {
	malformedDate := "not-a-date"
	s.mockSwitchRepo.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(&repository.Switch{
			ID:         "sw1",
			Visibility: repository.VisibilityPrivate,
			Purchases:  repository.BulkPurchases{{OrderDate: &malformedDate}},
		}, nil)

	req := s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear"}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteSwitchSuite struct {
	suite.Suite

	mockSwitches     *mocks.MockSwitchRepository
	mockBuilds       *mocks.MockBuildRepository
	mockBuildImages  *mocks.MockBuildImageStore
	mockSwitchImages *mocks.MockSwitchImageStore
	handler          http.HandlerFunc
}

func TestDeleteSwitchSuite(t *testing.T) {
	suite.Run(t, new(DeleteSwitchSuite))
}

func (s *DeleteSwitchSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = DeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
}

func (s *DeleteSwitchSuite) newRequest(ctx context.Context, onDelete string) *http.Request {
	target := "/users/alice/switches/sw1"
	if onDelete != "" {
		target += "?on_delete=" + onDelete
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, target, nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *DeleteSwitchSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_DefaultOnDelete_NoReferences_Returns204() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_Block_Referenced_Returns409WithBlockingBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
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

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_Cascade_Referenced_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
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

func (s *DeleteSwitchSuite) TestDeleteSwitch_Owner_InvalidOnDelete_Returns400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "bogus"))

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_Anonymous_Returns404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), ""))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_RepositoryError_Returns500() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockSwitches.EXPECT().
		Delete(mock.Anything, "sw1").
		Return(errors.New("delete item failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchSuite) TestDeleteSwitch_ImageDeleteFails_Returns500_DoesNotDeleteSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, "alice", "sw1").
		Return(nil, nil)
	key := repository.SwitchImageKey("switches/alice/sw1/image/up1")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &key}, nil)
	s.mockSwitchImages.EXPECT().
		Delete(mock.Anything, key).
		Return(errors.New("s3 unavailable"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), ""))

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockSwitches has no .EXPECT() for Delete - verifies the DB record
	// was never touched, so a retry can safely re-attempt the S3 delete.
}

type SetSwitchImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockSwitchRepository
	mockImages  *mocks.MockSwitchImageStore
	mockUploads *mocks.MockUploadCounter
	handler     http.HandlerFunc
}

func TestSetSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(SetSwitchImageSuite))
}

func (s *SetSwitchImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
	s.handler = SetSwitchImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
}

func (s *SetSwitchImageSuite) newRequest(ctx context.Context, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/users/alice/switches/sw1/image", strings.NewReader(body))
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *SetSwitchImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

const previousSwitchImageTestKey = repository.SwitchImageKey("switches/alice/sw1/image/old-upload")

// newSwitchImageKey matches a freshly minted per-upload key for sw1, which
// must never be the previous image's key.
func newSwitchImageKey() any {
	return mock.MatchedBy(func(k repository.SwitchImageKey) bool {
		return strings.HasPrefix(string(k), "switches/alice/sw1/image/") && k != previousSwitchImageTestKey
	})
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_Succeeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	var presigned repository.SwitchImageKey
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.SwitchImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/presigned-put", nil).
		NotBefore(reserved)
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", newSwitchImageKey()).
		Run(func(_ context.Context, _ string, key repository.SwitchImageKey) { s.Equal(presigned, key) }).
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

func (s *SetSwitchImageSuite) TestSetSwitchImage_ReplacingImage_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	deleted := s.mockImages.EXPECT().
		Delete(mock.Anything, previousSwitchImageTestKey).
		Return(nil).
		Call
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", newSwitchImageKey()).
		Return(nil).
		NotBefore(deleted)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx, `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_InvalidBody_Returns400() {
	req := s.newRequest(s.ownerCtx(), "not json")
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_UnapprovedContentType_Returns400() {
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

func (s *SetSwitchImageSuite) TestSetSwitchImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_PresignError_Returns500() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// No .EXPECT() for Delete or SetImagePath - verifies neither the
	// previous object nor the DB was touched when presigning fails.
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_DeletePreviousError_Returns500() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageTestKey
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, previousSwitchImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// No .EXPECT() for SetImagePath - the pointer stays on the previous
	// image, which still exists.
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_RepositoryError_Returns500() {
	expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockRepo.EXPECT().
		SetImagePath(mock.Anything, "sw1", newSwitchImageKey()).
		Return(errors.New("put item failed"))

	req := s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteSwitchImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockSwitchRepository
	mockImages *mocks.MockSwitchImageStore
	handler    http.HandlerFunc
}

func TestDeleteSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteSwitchImageSuite))
}

func (s *DeleteSwitchImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.handler = DeleteSwitchImage(s.mockRepo, s.mockImages)
}

func (s *DeleteSwitchImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/users/alice/switches/sw1/image", nil)
	req.SetPathValue("userId", "alice")
	req.SetPathValue("switchId", "sw1")
	return req
}

func (s *DeleteSwitchImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

var deleteSwitchImageTestKey = repository.SwitchImageKey("switches/alice/sw1/image/up1")

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearImagePath(mock.Anything, "sw1").
		Return(&deleteSwitchImageTestKey, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_ClearImagePathNotFound_Returns204() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(nil)
	s.mockRepo.EXPECT().
		ClearImagePath(mock.Anything, "sw1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_AlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1"}, nil)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_NotOwner_Returns404() {
	ctx := apictx.WithUserID(s.T().Context(), "bob")

	req := s.newRequest(ctx)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_Anonymous_Returns404() {
	req := s.newRequest(s.T().Context())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(nil, errors.New("get item failed"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteSwitchImageSuite) TestDeleteSwitchImage_S3DeleteError_Returns500_DoesNotDeleteDBRecord() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{ID: "sw1", ImagePath: &deleteSwitchImageTestKey}, nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, deleteSwitchImageTestKey).
		Return(errors.New("s3: access denied"))

	req := s.newRequest(s.ownerCtx())
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	// mockRepo has no .EXPECT() for ClearImagePath - verifies the DB
	// record was never touched, so a retry can safely re-attempt the S3
	// delete.
}

func (s *CreateSwitchSuite) TestCreateSwitch_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockSwitchRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`))

	s.Equal(http.StatusCreated, rec.Code)
	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchases)
	s.Require().Len(*got.Purchases, 1)
	s.Require().NotNil((*got.Purchases)[0].Currency)
	s.Equal("EUR", *(*got.Purchases)[0].Currency)
}

func (s *UpdateSwitchSuite) TestUpdateSwitch_ReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockSwitchRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Purchases: repository.BulkPurchases{{Price: &price}}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`))

	s.Equal(http.StatusOK, rec.Code)
	var got api.Switch
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Purchases)
	s.Require().Len(*got.Purchases, 1)
	s.Require().NotNil((*got.Purchases)[0].Currency)
	s.Equal("EUR", *(*got.Purchases)[0].Currency)
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_UploadLimitReached_Returns429() {
	previous := previousSwitchImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(&repository.Switch{ID: "sw1", ImagePath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusTooManyRequests, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	s.Contains(rec.Body.String(), "image uploads aren't available at the moment")
}

func (s *SetSwitchImageSuite) TestSetSwitchImage_ReserveError_Returns500() {
	previous := previousSwitchImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(&repository.Switch{ID: "sw1", ImagePath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}
