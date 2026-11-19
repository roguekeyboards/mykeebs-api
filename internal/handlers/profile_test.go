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

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type GetProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestGetProfileSuite(t *testing.T) {
	suite.Run(t, new(GetProfileSuite))
}

func (s *GetProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = GetProfile(s.mockRepo, repoapi.Profile{Images: s.mockImages})
}

func (s *GetProfileSuite) newRequest(ctx context.Context, identifier string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/v1/profile/"+identifier, nil)
	req.SetPathValue("identifier", identifier)
	return req
}

func (s *GetProfileSuite) TestDiscoverableByID_200() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: true, Bio: strp("keebs")}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusOK, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
	s.Equal("user-alice", body.UserId)
	s.Require().NotNil(body.Bio)
	s.Equal("keebs", *body.Bio)
	s.Nil(body.Preferences)
}

func (s *GetProfileSuite) TestDiscoverableByUsername_200() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "alice").Return("user-alice", nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: true}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "alice"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *GetProfileSuite) TestNonDiscoverable_Owner_200() {
	ctx := apictx.WithUserID(s.T().Context(), "user-alice")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{
			OwnerID: "user-alice", Username: "alice", Discoverable: false,
			Preferences: repository.ProfilePreferences{Currency: "EUR"},
		}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "user-alice"))

	s.Equal(http.StatusOK, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().NotNil(body.Preferences)
	s.Equal("EUR", body.Preferences.Currency)
}

func (s *GetProfileSuite) TestNonDiscoverable_OtherCaller_404() {
	ctx := apictx.WithUserID(s.T().Context(), "user-bob")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: false}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "user-alice"))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetProfileSuite) TestNonDiscoverable_Anonymous_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: false}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetProfileSuite) TestNoSuchProfile_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "ghost").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "ghost").Return("", repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "ghost"))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetProfileSuite) TestStoreError_500() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, errors.New("dynamo down"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice"))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func strp(v string) *string { return &v }

type CreateProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestCreateProfileSuite(t *testing.T) {
	suite.Run(t, new(CreateProfileSuite))
}

func (s *CreateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = CreateProfile(s.mockRepo, repoapi.Profile{Images: s.mockImages})
}

const profileUserID = "user-alice"

func (s *CreateProfileSuite) post(caller string, body any) *httptest.ResponseRecorder {
	s.T().Helper()

	var raw []byte
	switch b := body.(type) {
	case string:
		raw = []byte(b)
	default:
		var err error
		raw, err = json.Marshal(b)
		s.Require().NoError(err)
	}

	ctx := s.T().Context()
	if caller != "" {
		ctx = apictx.WithUserID(ctx, caller)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/profile/"+profileUserID, strings.NewReader(string(raw)))
	req.SetPathValue("identifier", profileUserID)

	rec := httptest.NewRecorder()
	s.handler(rec, req)
	return rec
}

func validInput() api.ProfileInput {
	return api.ProfileInput{Username: "alice", Preferences: validPreferences()}
}

func validPreferences() *api.ProfilePreferences {
	return &api.ProfilePreferences{Currency: "USD", ShowPriceToMe: true, ShowPriceToOthers: false}
}

func (s *CreateProfileSuite) TestValidInput_201() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{OwnerID: "user-alice", Username: "alice"}, nil)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusCreated, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
}

func (s *CreateProfileSuite) TestNotOwner_404_NoRepoCall() {
	rec := s.post("user-bob", validInput())

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateProfileSuite) TestAnonymous_404() {
	rec := s.post("", validInput())

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *CreateProfileSuite) TestMalformedBody_400() {
	rec := s.post("user-alice", "{not json")

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateProfileSuite) TestInvalidUsername_400_InvalidParams() {
	in := validInput()
	in.Username = "AB"
	rec := s.post("user-alice", in)

	s.Equal(http.StatusBadRequest, rec.Code)
	var body struct {
		InvalidParams []struct {
			Name string `json:"name"`
		} `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("username", body.InvalidParams[0].Name)
}

func (s *CreateProfileSuite) TestLinkHTTPURL_400_IndexedParam() {
	in := validInput()
	in.Links = &[]api.ProfileLink{{Name: "site", Url: "http://x.example"}}
	rec := s.post("user-alice", in)

	s.Equal(http.StatusBadRequest, rec.Code)
	var body struct {
		InvalidParams []struct {
			Name string `json:"name"`
		} `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("links[0].url", body.InvalidParams[0].Name)
}

func (s *CreateProfileSuite) TestAlreadyHasProfile_409_ConflictType() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type string `json:"type"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/conflict", body.Type)
}

func (s *CreateProfileSuite) TestUsernameTaken_409_UsernameUnavailableType() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/username-unavailable", body.Type)
	s.Equal(`the username "alice" is already taken`, body.Detail)
}

func (s *CreateProfileSuite) TestMutationConflict_409_ConflictType() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type string `json:"type"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/conflict", body.Type)
}

func (s *CreateProfileSuite) TestRepoError_500() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	rec := s.post("user-alice", validInput())

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type UpdateProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestUpdateProfileSuite(t *testing.T) {
	suite.Run(t, new(UpdateProfileSuite))
}

func (s *UpdateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = UpdateProfile(s.mockRepo, repoapi.Profile{Images: s.mockImages})
}

func (s *UpdateProfileSuite) put(caller string, body any) *httptest.ResponseRecorder {
	s.T().Helper()

	var raw []byte
	switch b := body.(type) {
	case string:
		raw = []byte(b)
	default:
		var err error
		raw, err = json.Marshal(b)
		s.Require().NoError(err)
	}

	ctx := s.T().Context()
	if caller != "" {
		ctx = apictx.WithUserID(ctx, caller)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodPut, "/v1/profile/"+profileUserID, strings.NewReader(string(raw)))
	req.SetPathValue("identifier", profileUserID)

	rec := httptest.NewRecorder()
	s.handler(rec, req)
	return rec
}

func (s *UpdateProfileSuite) TestValidInput_200() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{OwnerID: "user-alice", Username: "alice"}, nil)

	rec := s.put("user-alice", api.ProfileInput{Username: "alice", Preferences: validPreferences()})

	s.Equal(http.StatusOK, rec.Code)
	var body api.Profile
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("alice", body.Username)
	s.Equal("user-alice", body.UserId)
}

func (s *UpdateProfileSuite) TestNotOwner_404_NoRepoCall() {
	rec := s.put("user-bob", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *UpdateProfileSuite) TestAnonymous_404() {
	rec := s.put("", api.ProfileInput{Username: "alice"})

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateProfileSuite) TestMalformedBody_400() {
	rec := s.put("user-alice", "{not json")

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpdateProfileSuite) TestInvalidUsername_400() {
	rec := s.put("user-alice", api.ProfileInput{Username: "AB"})

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpdateProfileSuite) TestNoProfile_404() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	rec := s.put("user-alice", api.ProfileInput{Username: "alice", Preferences: validPreferences()})

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateProfileSuite) TestUsernameTaken_409_UsernameUnavailableType() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	rec := s.put("user-alice", api.ProfileInput{Username: "taken", Preferences: validPreferences()})

	s.Equal(http.StatusConflict, rec.Code)
	var body struct {
		Type   string `json:"type"`
		Detail string `json:"detail"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Equal("https://mykeebs.info/errors/username-unavailable", body.Type)
	s.Equal(`the username "taken" is already taken`, body.Detail)
}

func (s *UpdateProfileSuite) TestRepoError_500() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	rec := s.put("user-alice", api.ProfileInput{Username: "alice", Preferences: validPreferences()})

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *UpdateProfileSuite) TestMutationConflict_409() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	rec := s.put("user-alice", api.ProfileInput{Username: "alice", Preferences: validPreferences()})

	s.Equal(http.StatusConflict, rec.Code)
}

type DeleteProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockOwnerImageStore
	handler    http.HandlerFunc
}

func TestDeleteProfileSuite(t *testing.T) {
	suite.Run(t, new(DeleteProfileSuite))
}

func (s *DeleteProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockOwnerImageStore(s.T())
	s.handler = DeleteProfile(accountdelete.Account{
		Images:     s.mockImages,
		Builds:     emptyItemStore{},
		Keyboards:  emptyItemStore{},
		Switches:   emptyItemStore{},
		ECParts:    emptyItemStore{},
		KeycapSets: emptyItemStore{},
		Profiles:   s.mockRepo,
	})
}

type emptyItemStore struct{}

func (emptyItemStore) DeleteAll(context.Context) error { return nil }

func (s *DeleteProfileSuite) del(caller string) *httptest.ResponseRecorder {
	s.T().Helper()

	ctx := s.T().Context()
	if caller != "" {
		ctx = apictx.WithUserID(ctx, caller)
	}

	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/profile/"+profileUserID, nil)
	req.SetPathValue("identifier", profileUserID)

	rec := httptest.NewRecorder()
	s.handler(rec, req)
	return rec
}

func (s *DeleteProfileSuite) TestDeletesAccount_204() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything).Return(nil)

	rec := s.del("user-alice")

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteProfileSuite) TestNotOwner_404_NoRepoCall() {
	rec := s.del("user-bob")

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DeleteProfileSuite) TestAnonymous_404() {
	rec := s.del("")

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DeleteProfileSuite) TestDeleteFails_500() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(errors.New("s3 down"))

	rec := s.del("user-alice")

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *DeleteProfileSuite) TestMutationConflict_409() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything).Return(repository.ErrMutationConflict)

	rec := s.del("user-alice")

	s.Equal(http.StatusConflict, rec.Code)
}

type ListProfilesSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestListProfilesSuite(t *testing.T) {
	suite.Run(t, new(ListProfilesSuite))
}

func (s *ListProfilesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = ListProfiles(s.mockRepo, repoapi.Profile{Images: s.mockImages})
}

func (s *ListProfilesSuite) request(query string) *http.Request {
	return httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/profiles?"+query, nil)
}

func (s *ListProfilesSuite) TestNoFilters_PassesEmptyPrefixes() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "").
		Return([]repository.Profile{
			{OwnerID: "user-alice", Username: "alice", Discoverable: true},
		}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20"))

	s.Equal(http.StatusOK, rec.Code)
	var got api.ProfileListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 1)
	row := (*got.Items)[0]
	s.Equal("alice", row.Username)
	s.Equal("user-alice", row.UserId)
}

func (s *ListProfilesSuite) TestFullProfiles_PreferencesOnlyOnCallersOwnRow() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "").
		Return([]repository.Profile{
			{OwnerID: "user-alice", Username: "alice", Discoverable: true, Bio: strp("alice's keebs"), Preferences: repository.ProfilePreferences{Currency: "EUR"}},
			{OwnerID: "user-bob", Username: "bob", Discoverable: true, Bio: strp("bob's keebs"), Preferences: repository.ProfilePreferences{Currency: "GBP"}},
		}, "", nil)

	req := httptest.NewRequestWithContext(apictx.WithUserID(s.T().Context(), "user-alice"), http.MethodGet, "/v1/profiles?limit=20", nil)
	rec := httptest.NewRecorder()
	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	var got api.ProfileListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.Items)
	s.Require().Len(*got.Items, 2)
	alice, bob := (*got.Items)[0], (*got.Items)[1]

	s.Require().NotNil(alice.Bio)
	s.Equal("alice's keebs", *alice.Bio)
	s.Require().NotNil(alice.Preferences)
	s.Equal("EUR", alice.Preferences.Currency)

	s.Require().NotNil(bob.Bio)
	s.Equal("bob's keebs", *bob.Bio)
	s.Nil(bob.Preferences)
}

func (s *ListProfilesSuite) TestUsernameFilter_Forwarded() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "al", "", 20, "").
		Return([]repository.Profile{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20&username=al"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListProfilesSuite) TestDiscordFilter_Forwarded() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "cool", 20, "").
		Return([]repository.Profile{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20&discord_username=cool"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListProfilesSuite) TestBothFilters_400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.request("username=al&discord_username=cool"))

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ListProfilesSuite) TestPassesLimitAndCursor() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 5, "abc").
		Return([]repository.Profile{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=5&cursor=abc"))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListProfilesSuite) TestReturnsNextCursor_WhenPresent() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "").
		Return([]repository.Profile{}, "next-page", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20"))

	var got api.ProfileListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().NotNil(got.NextCursor)
	s.Equal("next-page", *got.NextCursor)
}

func (s *ListProfilesSuite) TestAvatarSigned_WhenSet() {
	key := repository.ProfileImageKey("profiles/user-alice/avatar/up1")
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "").
		Return([]repository.Profile{
			{OwnerID: "user-alice", Username: "alice", Discoverable: true, AvatarPath: &key},
		}, "", nil)
	s.mockImages.EXPECT().SignGet(mock.Anything, key).Return("https://signed/avatar", nil)
	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20"))

	var got api.ProfileListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	row := (*got.Items)[0]
	s.Require().NotNil(row.Avatar)
	s.Equal("https://signed/avatar", *row.Avatar.Url)
}

func (s *ListProfilesSuite) TestRepositoryError_500() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "").
		Return(nil, "", errors.New("query failed"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20"))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *ListProfilesSuite) TestInvalidCursor_400() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", 20, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	rec := httptest.NewRecorder()
	s.handler(rec, s.request("limit=20&cursor=stale"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type SetProfileImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockProfileRepository
	mockImages  *mocks.MockProfileImageStore
	mockUploads *mocks.MockUploadCounter
	handler     http.HandlerFunc
}

func TestSetProfileImageSuite(t *testing.T) {
	suite.Run(t, new(SetProfileImageSuite))
}

func (s *SetProfileImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
	s.handler = SetProfileImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
}

func (s *SetProfileImageSuite) newRequest(ctx context.Context, identifier, body string) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/profile/"+identifier+"/image", strings.NewReader(body))
	req.SetPathValue("identifier", identifier)
	return req
}

func (s *SetProfileImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "user-alice")
}

const previousProfileImageTestKey = repository.ProfileImageKey("profiles/user-alice/avatar/old-upload")

func newProfileImageKey() any {
	return mock.MatchedBy(func(k repository.ProfileImageKey) bool {
		return strings.HasPrefix(string(k), "profiles/user-alice/avatar/") && k != previousProfileImageTestKey
	})
}

func (s *SetProfileImageSuite) TestSucceeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(&repository.Profile{}, nil)
	var presigned repository.ProfileImageKey
	s.mockImages.EXPECT().PresignPut(mock.Anything, newProfileImageKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.ProfileImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/presigned-put", nil).
		NotBefore(reserved)
	s.mockRepo.EXPECT().SetAvatarPath(mock.Anything, newProfileImageKey()).
		Run(func(_ context.Context, key repository.ProfileImageKey) { s.Equal(presigned, key) }).
		Return(nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))
	var got struct {
		UploadURL string `json:"upload_url"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("https://example.com/presigned-put", got.UploadURL)
}

func (s *SetProfileImageSuite) TestReplacingAvatar_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousProfileImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newProfileImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, previousProfileImageTestKey).Return(nil).Call
	s.mockRepo.EXPECT().SetAvatarPath(mock.Anything, newProfileImageKey()).Return(nil).NotBefore(deleted)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *SetProfileImageSuite) TestNotOwner_404() {
	ctx := apictx.WithUserID(s.T().Context(), "user-bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx, "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetProfileImageSuite) TestUsernameIdentifier_404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *SetProfileImageSuite) TestAnonymous_404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *SetProfileImageSuite) TestInvalidBody_400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", "not json"))

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetProfileImageSuite) TestUnapprovedContentType_400() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"application/pdf"}`))

	s.Equal(http.StatusBadRequest, rec.Code)
	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("content_type", got.InvalidParams[0].Name)
}

func (s *SetProfileImageSuite) TestNoProfile_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *SetProfileImageSuite) TestPresignError_500() {
	expectUploadReserved(s.mockUploads)
	previous := previousProfileImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newProfileImageKey(), "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *SetProfileImageSuite) TestDeletePreviousError_500() {
	expectUploadReserved(s.mockUploads)
	previous := previousProfileImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newProfileImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/presigned-put", nil)
	s.mockImages.EXPECT().Delete(mock.Anything, previousProfileImageTestKey).Return(errors.New("s3: access denied"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type DeleteProfileImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
	handler    http.HandlerFunc
}

func TestDeleteProfileImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteProfileImageSuite))
}

func (s *DeleteProfileImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.handler = DeleteProfileImage(s.mockRepo, s.mockImages)
}

func (s *DeleteProfileImageSuite) newRequest(ctx context.Context) *http.Request {
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/v1/profile/user-alice/image", nil)
	req.SetPathValue("identifier", "user-alice")
	return req
}

func (s *DeleteProfileImageSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "user-alice")
}

var deleteProfileImageTestKey = repository.ProfileImageKey("profiles/user-alice/avatar/up1")

func (s *DeleteProfileImageSuite) TestSucceeds() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", AvatarPath: &deleteProfileImageTestKey}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, deleteProfileImageTestKey).Return(nil)
	s.mockRepo.EXPECT().ClearAvatarPath(mock.Anything).Return(&deleteProfileImageTestKey, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteProfileImageSuite) TestClearAvatarPathNotFound_204() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", AvatarPath: &deleteProfileImageTestKey}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, deleteProfileImageTestKey).Return(nil)
	s.mockRepo.EXPECT().ClearAvatarPath(mock.Anything).Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteProfileImageSuite) TestNoAvatar_204_WithoutS3Call() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice"}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *DeleteProfileImageSuite) TestNotOwner_404() {
	ctx := apictx.WithUserID(s.T().Context(), "user-bob")

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(ctx))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteProfileImageSuite) TestAnonymous_404() {
	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.T().Context()))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *DeleteProfileImageSuite) TestNoProfile_404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteProfileImageSuite) TestS3DeleteError_500_DoesNotClearDBRecord() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", AvatarPath: &deleteProfileImageTestKey}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, deleteProfileImageTestKey).
		Return(errors.New("s3: access denied"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx()))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *SetProfileImageSuite) TestUploadLimitReached_Returns429() {
	previous := previousProfileImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusTooManyRequests, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
	s.Contains(rec.Body.String(), "image uploads aren't available at the moment")
}

func (s *SetProfileImageSuite) TestReserveError_Returns500() {
	previous := previousProfileImageTestKey
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	rec := httptest.NewRecorder()
	s.handler(rec, s.newRequest(s.ownerCtx(), "user-alice", `{"content_type":"image/webp","size_bytes":524288}`))

	s.Equal(http.StatusInternalServerError, rec.Code)
}
