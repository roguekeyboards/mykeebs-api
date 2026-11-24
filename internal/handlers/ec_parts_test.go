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

type ecPartRequest struct {
	method, query, body string
	pathID              string
	prefs               repository.ProfilePreferences
}

func newECPartRequest(ctx context.Context, r ecPartRequest) *http.Request {
	ctx = ownerprefs.WithPreferences(ctx, r.prefs)
	path := "/users/alice/ec-parts"
	if r.pathID != "" {
		path += "/" + r.pathID
	}
	if r.query != "" {
		path += "?" + r.query
	}
	req := httptest.NewRequestWithContext(ctx, r.method, path, strings.NewReader(r.body))
	req.SetPathValue("userId", "alice")
	if r.pathID != "" {
		req.SetPathValue("ecPartId", r.pathID)
	}
	return req
}

type ListECPartsSuite struct {
	suite.Suite

	mockRepo *mocks.MockECPartRepository
	handler  http.HandlerFunc
}

func TestListECPartsSuite(t *testing.T) {
	suite.Run(t, new(ListECPartsSuite))
}

func (s *ListECPartsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.handler = ListECParts(s.mockRepo, repoapi.ECPart{})
}

func (s *ListECPartsSuite) TestOwner_ListsAllVisibilities() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.MatchedBy(func(vis []repository.Visibility) bool { return len(vis) == 3 }), repository.ECPartType(""), 20, "").
		Return([]repository.ECPart{{ID: "p1", Brand: "DynaCap", Name: "Heavy", PartType: repository.ECPartTypeDomes, Visibility: repository.VisibilityPrivate}}, "next", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(apictx.WithUserID(s.T().Context(), "alice"), ecPartRequest{method: http.MethodGet, query: "limit=20"}))

	s.Equal(http.StatusOK, rec.Code)
	var got api.ECPartListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(&[]api.ECPart{{Id: "p1", Brand: "DynaCap", Name: "Heavy", PartType: api.ECPartTypeDomes, Visibility: new(api.VisibilityPrivate)}}, got.Items)
	s.Equal(new("next"), got.NextCursor)
}

func (s *ListECPartsSuite) TestPartTypeQuery_PassedToRepository() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", []repository.Visibility{repository.VisibilityPublic}, repository.ECPartTypeSprings, 20, "c1").
		Return([]repository.ECPart{}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, query: "part_type=springs&limit=20&cursor=c1"}))

	s.Equal(http.StatusOK, rec.Code)
}

func (s *ListECPartsSuite) TestOwnerShowPriceToMeFalse_OmitsPrice() {
	s.mockRepo.EXPECT().
		List(mock.Anything, "alice", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.ECPart{{ID: "p1", Purchases: repository.BulkPurchases{{Price: new(36.0), Vendor: new("Omnitype")}}}}, "", nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(apictx.WithUserID(s.T().Context(), "alice"), ecPartRequest{method: http.MethodGet, query: "limit=20"}))

	var got api.ECPartListPage
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(*got.Items, 1)
	s.Equal(&api.ECPartPurchase{Vendor: new("Omnitype")}, (*got.Items)[0].Purchase)
}

func (s *ListECPartsSuite) TestInvalidCursor_Returns400() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", repository.ErrInvalidCursor)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, query: "limit=20&cursor=bad"}))

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *ListECPartsSuite) TestRepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("dynamodb down"))

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, query: "limit=20"}))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type GetECPartSuite struct {
	suite.Suite

	mockRepo *mocks.MockECPartRepository
	handler  http.HandlerFunc
}

func TestGetECPartSuite(t *testing.T) {
	suite.Run(t, new(GetECPartSuite))
}

func (s *GetECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.handler = GetECPart(s.mockRepo, repoapi.ECPart{})
}

func (s *GetECPartSuite) TestOwner_Succeeds() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", PartType: repository.ECPartTypeRings, Rings: &repository.ECRings{Thickness: new(0.5)}, Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(apictx.WithUserID(s.T().Context(), "alice"), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusOK, rec.Code)
	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(api.ECPart{Id: "p1", PartType: api.ECPartTypeRings, Rings: &api.ECRings{Thickness: new(0.5)}, Visibility: new(api.VisibilityPrivate)}, got)
}

func (s *GetECPartSuite) TestAnonymousReadingPrivate_Returns404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPrivate}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetECPartSuite) TestAnonymousReadingPublic_SucceedsWithoutVisibility() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusOK, rec.Code)
	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Nil(got.Visibility)
}

func (s *GetECPartSuite) TestNotFound_Returns404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(nil, repository.ErrNotFound)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *GetECPartSuite) TestMalformedStoredDate_Returns500() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic, Purchases: repository.BulkPurchases{{OrderDate: new("bad")}}}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type CreateECPartSuite struct {
	suite.Suite

	mockRepo *mocks.MockECPartRepository
	handler  http.HandlerFunc
}

func TestCreateECPartSuite(t *testing.T) {
	suite.Run(t, new(CreateECPartSuite))
}

func (s *CreateECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.handler = CreateECPart(s.mockRepo, repoapi.ECPart{})
}

func (s *CreateECPartSuite) post(ctx context.Context, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(ctx, ecPartRequest{method: http.MethodPost, body: body}))
	return rec
}

func (s *CreateECPartSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *CreateECPartSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(p repository.ECPart) bool {
			return p.ID != "" && p.PartType == repository.ECPartTypeDomes && p.Domes != nil && *p.Domes.Weight == 55
		})).
		RunAndReturn(func(_ context.Context, p repository.ECPart) (*repository.ECPart, error) { return &p, nil })

	rec := s.post(s.ownerCtx(), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","domes":{"weight":55},"visibility":"private"}`)

	s.Equal(http.StatusCreated, rec.Code)
	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.NotEmpty(got.Id)
	s.Equal(&api.ECDomes{Weight: new(55.0)}, got.Domes)
}

func (s *CreateECPartSuite) TestOtherTypesSubObject_Returns400NamingIt() {
	rec := s.post(s.ownerCtx(), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","sliders":{"size":"1U"},"visibility":"private"}`)

	s.Equal(http.StatusBadRequest, rec.Code)
	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("sliders", got.InvalidParams[0].Name)
}

func (s *CreateECPartSuite) TestUnapprovedMaterial_Returns400() {
	rec := s.post(s.ownerCtx(), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","domes":{"material":"Steel"},"visibility":"private"}`)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateECPartSuite) TestNotOwner_Returns404() {
	rec := s.post(apictx.WithUserID(s.T().Context(), "bob"), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private"}`)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *CreateECPartSuite) TestInvalidBody_Returns400() {
	rec := s.post(s.ownerCtx(), `{`)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateECPartSuite) TestAlreadyExists_Returns409() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, repository.ErrAlreadyExists)

	rec := s.post(s.ownerCtx(), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private"}`)

	s.Equal(http.StatusConflict, rec.Code)
}

func (s *CreateECPartSuite) TestRepositoryError_Returns500() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.New("dynamodb down"))

	rec := s.post(s.ownerCtx(), `{"brand":"DynaCap","name":"Heavy","part_type":"domes","visibility":"private"}`)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

type UpdateECPartSuite struct {
	suite.Suite

	mockRepo *mocks.MockECPartRepository
	handler  http.HandlerFunc
}

func TestUpdateECPartSuite(t *testing.T) {
	suite.Run(t, new(UpdateECPartSuite))
}

func (s *UpdateECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.handler = UpdateECPart(s.mockRepo, repoapi.ECPart{})
}

func (s *UpdateECPartSuite) put(ctx context.Context, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(ctx, ecPartRequest{method: http.MethodPut, pathID: "p1", body: body}))
	return rec
}

func (s *UpdateECPartSuite) ownerCtx() context.Context {
	return apictx.WithUserID(s.T().Context(), "alice")
}

func (s *UpdateECPartSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(p repository.ECPart) bool {
			return p.ID == "p1" && p.PartType == repository.ECPartTypeSprings && p.Springs != nil
		})).
		RunAndReturn(func(_ context.Context, p repository.ECPart) (*repository.ECPart, error) { return &p, nil })

	rec := s.put(s.ownerCtx(), `{"brand":"DynaCap","name":"Springs","part_type":"springs","springs":{"material":"Nickel-Plated"},"visibility":"public"}`)

	s.Equal(http.StatusOK, rec.Code)
	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("p1", got.Id)
}

func (s *UpdateECPartSuite) TestOtherTypesSubObject_Returns400() {
	rec := s.put(s.ownerCtx(), `{"brand":"DynaCap","name":"Springs","part_type":"springs","rings":{},"visibility":"public"}`)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *UpdateECPartSuite) TestNotOwner_Returns404() {
	rec := s.put(apictx.WithUserID(s.T().Context(), "bob"), `{"brand":"DynaCap","name":"Springs","part_type":"springs","visibility":"public"}`)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateECPartSuite) TestNotFound_Returns404() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)

	rec := s.put(s.ownerCtx(), `{"brand":"DynaCap","name":"Springs","part_type":"springs","visibility":"public"}`)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *UpdateECPartSuite) TestRepositoryError_Returns500() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, errors.New("dynamodb down"))

	rec := s.put(s.ownerCtx(), `{"brand":"DynaCap","name":"Springs","part_type":"springs","visibility":"public"}`)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *GetECPartSuite) TestApprovedImage_SignedURLIncluded() {
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/u1")
	images := mocks.NewMockECPartImageStore(s.T())
	images.EXPECT().SignGet(mock.Anything, key).Return("https://images.example/p1", nil)
	s.handler = GetECPart(s.mockRepo, repoapi.ECPart{Images: images})
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic, ImagePath: &key, ImageStatus: repository.ImageStatusApproved}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	s.Equal(http.StatusOK, rec.Code)
	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(&api.ECPartImage{Url: new("https://images.example/p1")}, got.Image)
}

func (s *GetECPartSuite) TestPendingImage_HiddenFromOthers() {
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/u1")
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPublic, ImagePath: &key, ImageStatus: repository.ImageStatusPending}, nil)

	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(s.T().Context(), ecPartRequest{method: http.MethodGet, pathID: "p1"}))

	var got api.ECPart
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Nil(got.Image)
}

type SetECPartImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockECPartRepository
	mockImages  *mocks.MockECPartImageStore
	mockUploads *mocks.MockUploadCounter
	handler     http.HandlerFunc
}

func TestSetECPartImageSuite(t *testing.T) {
	suite.Run(t, new(SetECPartImageSuite))
}

func (s *SetECPartImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
	s.handler = SetECPartImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
}

func (s *SetECPartImageSuite) post(ctx context.Context, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(ctx, ecPartRequest{method: http.MethodPost, pathID: "p1", body: body}))
	return rec
}

const previousECPartImageKey = repository.ECPartImageKey("ec-parts/alice/p1/image/old-upload")

func newECPartImageKey() any {
	return mock.MatchedBy(func(k repository.ECPartImageKey) bool {
		return strings.HasPrefix(string(k), "ec-parts/alice/p1/image/") && k != previousECPartImageKey
	})
}

func (s *SetECPartImageSuite) TestReplacingImage_PresignsThenDeletesPreviousThenSetsPath() {
	reserved := expectUploadReserved(s.mockUploads)
	previous := previousECPartImageKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &previous}, nil)
	presigned := s.mockImages.EXPECT().
		PresignPut(mock.Anything, newECPartImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/put", nil).
		NotBefore(reserved)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, previousECPartImageKey).Return(nil).NotBefore(presigned)
	s.mockRepo.EXPECT().SetImagePath(mock.Anything, "p1", newECPartImageKey()).Return(nil).NotBefore(deleted)

	rec := s.post(apictx.WithUserID(s.T().Context(), "alice"), `{"content_type":"image/webp","size_bytes":524288}`)

	s.Equal(http.StatusCreated, rec.Code)
	var got api.ECPartImageUpload
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("https://example.com/put", got.UploadUrl)
}

func (s *SetECPartImageSuite) TestNoPreviousImage_SetsPathWithoutDeleting() {
	expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newECPartImageKey(), "image/webp", int64(524288)).Return("https://example.com/put", nil)
	s.mockRepo.EXPECT().SetImagePath(mock.Anything, "p1", newECPartImageKey()).Return(nil)

	rec := s.post(apictx.WithUserID(s.T().Context(), "alice"), `{"content_type":"image/webp","size_bytes":524288}`)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *SetECPartImageSuite) TestPresignFails_Returns500AndKeepsPreviousImage() {
	expectUploadReserved(s.mockUploads)
	previous := previousECPartImageKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("presign failed"))

	rec := s.post(apictx.WithUserID(s.T().Context(), "alice"), `{"content_type":"image/webp","size_bytes":524288}`)

	s.Equal(http.StatusInternalServerError, rec.Code)
}

func (s *SetECPartImageSuite) TestUnapprovedContentType_Returns400() {
	rec := s.post(apictx.WithUserID(s.T().Context(), "alice"), `{"content_type":"image/png","size_bytes":10}`)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *SetECPartImageSuite) TestNotOwner_Returns404() {
	rec := s.post(apictx.WithUserID(s.T().Context(), "bob"), `{"content_type":"image/webp","size_bytes":10}`)

	s.Equal(http.StatusNotFound, rec.Code)
}

func (s *SetECPartImageSuite) TestPartNotFound_Returns404() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(nil, repository.ErrNotFound)

	rec := s.post(apictx.WithUserID(s.T().Context(), "alice"), `{"content_type":"image/webp","size_bytes":10}`)

	s.Equal(http.StatusNotFound, rec.Code)
}

type DeleteECPartImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockECPartRepository
	mockImages *mocks.MockECPartImageStore
	handler    http.HandlerFunc
}

func TestDeleteECPartImageSuite(t *testing.T) {
	suite.Run(t, new(DeleteECPartImageSuite))
}

func (s *DeleteECPartImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
	s.handler = DeleteECPartImage(s.mockRepo, s.mockImages)
}

func (s *DeleteECPartImageSuite) del(ctx context.Context) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(ctx, ecPartRequest{method: http.MethodDelete, pathID: "p1"}))
	return rec
}

func (s *DeleteECPartImageSuite) TestDeletesObjectThenClearsPath() {
	key := previousECPartImageKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &key}, nil)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil).Call
	s.mockRepo.EXPECT().ClearImagePath(mock.Anything, "p1").Return(&key, nil).NotBefore(deleted)

	s.Equal(http.StatusNoContent, s.del(apictx.WithUserID(s.T().Context(), "alice")).Code)
}

func (s *DeleteECPartImageSuite) TestNoImage_Returns204WithoutDeleting() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1"}, nil)

	s.Equal(http.StatusNoContent, s.del(apictx.WithUserID(s.T().Context(), "alice")).Code)
}

func (s *DeleteECPartImageSuite) TestObjectDeleteFails_Returns500AndKeepsPath() {
	key := previousECPartImageKey
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 down"))

	s.Equal(http.StatusInternalServerError, s.del(apictx.WithUserID(s.T().Context(), "alice")).Code)
}

func (s *DeleteECPartImageSuite) TestNotOwner_Returns404() {
	s.Equal(http.StatusNotFound, s.del(apictx.WithUserID(s.T().Context(), "bob")).Code)
}

type DeleteECPartSuite struct {
	suite.Suite

	mockRepo        *mocks.MockECPartRepository
	mockBuilds      *mocks.MockBuildRepository
	mockBuildImages *mocks.MockBuildImageStore
	mockImages      *mocks.MockECPartImageStore
	handler         http.HandlerFunc
}

func TestDeleteECPartSuite(t *testing.T) {
	suite.Run(t, new(DeleteECPartSuite))
}

func (s *DeleteECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
	s.handler = DeleteECPart(s.mockRepo, s.mockBuilds, s.mockBuildImages, s.mockImages)
}

func (s *DeleteECPartSuite) del(ctx context.Context, query string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.handler(rec, newECPartRequest(ctx, ecPartRequest{method: http.MethodDelete, pathID: "p1", query: query}))
	return rec
}

func (s *DeleteECPartSuite) TestUnreferenced_Returns204() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, "alice", "p1").Return(nil, nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "p1").Return(nil)

	s.Equal(http.StatusNoContent, s.del(apictx.WithUserID(s.T().Context(), "alice"), "on_delete=block").Code)
}

func (s *DeleteECPartSuite) TestBlocked_Returns409WithBuildIDs() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, "alice", "p1").Return([]string{"b1"}, nil)

	rec := s.del(apictx.WithUserID(s.T().Context(), "alice"), "on_delete=block")

	s.Equal(http.StatusConflict, rec.Code)
	s.Contains(rec.Body.String(), "b1")
}

func (s *DeleteECPartSuite) TestCascade_Returns200WithDeletedBuildIDs() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, "alice", "p1").Return([]string{"b1"}, nil)
	s.mockBuilds.EXPECT().Get(mock.Anything, "alice", "b1").Return(&repository.Build{ID: "b1"}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, "b1").Return(nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "p1").Return(nil)

	rec := s.del(apictx.WithUserID(s.T().Context(), "alice"), "on_delete=cascade")

	s.Equal(http.StatusOK, rec.Code)
	var got api.CascadeDeleteResult
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal([]string{"b1"}, got.DeletedBuildIds)
}

func (s *DeleteECPartSuite) TestInvalidOnDelete_Returns400() {
	s.Equal(http.StatusBadRequest, s.del(apictx.WithUserID(s.T().Context(), "alice"), "on_delete=detach").Code)
}

func (s *DeleteECPartSuite) TestNotOwner_Returns404() {
	s.Equal(http.StatusNotFound, s.del(apictx.WithUserID(s.T().Context(), "bob"), "on_delete=block").Code)
}

func (s *DeleteECPartSuite) TestRepositoryError_Returns500() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, "alice", "p1").Return(nil, errors.New("dynamodb down"))

	s.Equal(http.StatusInternalServerError, s.del(apictx.WithUserID(s.T().Context(), "alice"), "on_delete=block").Code)
}
