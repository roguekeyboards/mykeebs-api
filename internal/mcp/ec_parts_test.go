package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type HandleListECPartsSuite struct {
	suite.Suite

	mockRepo  *mocks.MockECPartRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleListECPartsSuite(t *testing.T) {
	suite.Run(t, new(HandleListECPartsSuite))
}

func (s *HandleListECPartsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleListECPartsSuite) TestOwnCollection_ListsEveryTierAndMapsSummaries() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic, repository.VisibilityAuthenticated, repository.VisibilityPrivate,
		}, repository.ECPartType(""), defaultListLimit, "").
		Return([]repository.ECPart{{ID: "p1", Brand: "DynaCap", Name: "Heavy", PartType: repository.ECPartTypeDomes, Visibility: repository.VisibilityPrivate}}, "next", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	_, out, err := handleListECParts(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.ListECPartsInput{})

	s.Require().NoError(err)
	s.Equal([]schema.ECPartSummary{{ID: "p1", Brand: "DynaCap", Name: "Heavy", PartType: "domes", Visibility: new("private")}}, out.ECParts)
	s.Equal("next", out.NextCursor)
}

func (s *HandleListECPartsSuite) TestPartType_PassedToRepository() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, repository.ECPartTypeRings, mock.Anything, "c1").
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	_, _, err := handleListECParts(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.ListECPartsInput{PartType: "rings", Cursor: "c1"})

	s.Require().NoError(err)
}

func (s *HandleListECPartsSuite) TestUnknownPartType_ReturnsErrorWithoutListing() {
	_, _, err := handleListECParts(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.ListECPartsInput{PartType: "keycaps"})

	s.EqualError(err, "part_type: must be one of: domes, sliders, housings, springs, rings")
}

func (s *HandleListECPartsSuite) TestInvalidCursor_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", repository.ErrInvalidCursor)

	_, _, err := handleListECParts(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.ListECPartsInput{Cursor: "bad"})

	s.ErrorContains(err, "invalid cursor")
}

type HandleGetECPartSuite struct {
	suite.Suite

	mockRepo  *mocks.MockECPartRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleGetECPartSuite(t *testing.T) {
	suite.Run(t, new(HandleGetECPartSuite))
}

func (s *HandleGetECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleGetECPartSuite) TestSucceeds() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").
		Return(&repository.ECPart{ID: "p1", PartType: repository.ECPartTypeSliders, Sliders: &repository.ECSliders{Stem: new(repository.ECSliderStemMX)}, Visibility: repository.VisibilityPrivate}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	_, out, err := handleGetECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.GetECPartInput{ECPartID: "p1"})

	s.Require().NoError(err)
	s.Equal(&schema.ECSliders{Stem: new("MX")}, out.ECPart.Sliders)
}

func (s *HandleGetECPartSuite) TestBlankID_ReturnsError() {
	_, _, err := handleGetECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.GetECPartInput{ECPartID: " "})

	s.ErrorContains(err, "ec_part_id")
}

func (s *HandleGetECPartSuite) TestOtherUsersPrivatePart_ReturnsNotFound() {
	s.mockRepo.EXPECT().Get(mock.Anything, "other", "p1").
		Return(&repository.ECPart{ID: "p1", Visibility: repository.VisibilityPrivate}, nil)

	_, _, err := handleGetECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.GetECPartInput{ECPartID: "p1", UserID: "other"})

	s.ErrorIs(err, errECPartNotFound)
}

type HandleCreateECPartSuite struct {
	suite.Suite

	mockRepo  *mocks.MockECPartRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleCreateECPartSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateECPartSuite))
}

func (s *HandleCreateECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func validECPartInput() schema.ECPartInput {
	return schema.ECPartInput{Brand: "DynaCap", Name: "Heavy", PartType: "domes", Domes: &schema.ECDomes{Weight: new(55.0)}, Visibility: "private"}
}

func (s *HandleCreateECPartSuite) TestSucceeds() {
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.mockRepo.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(p repository.ECPart) bool {
			return p.ID != "" && p.PartType == repository.ECPartTypeDomes && *p.Domes.Weight == 55
		})).
		RunAndReturn(func(_ context.Context, p repository.ECPart) (*repository.ECPart, error) { return &p, nil })

	_, out, err := handleCreateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateECPartInput{ECPartInput: validECPartInput()})

	s.Require().NoError(err)
	s.NotEmpty(out.ECPart.ID)
}

func (s *HandleCreateECPartSuite) TestInvalidInputs_ReturnErrorsWithoutCreating() {
	for name, tc := range map[string]struct {
		edit func(*schema.ECPartInput)
		want string
	}{
		"blank brand":       {func(in *schema.ECPartInput) { in.Brand = " " }, "brand"},
		"blank name":        {func(in *schema.ECPartInput) { in.Name = "" }, "name"},
		"bad date":          {func(in *schema.ECPartInput) { in.Purchase = &schema.ECPartPurchaseInput{OrderDate: new("21/09/2026")} }, "order_date"},
		"bad visibility":    {func(in *schema.ECPartInput) { in.Visibility = "friends" }, "visibility"},
		"unknown part_type": {func(in *schema.ECPartInput) { in.PartType = "keycaps" }, "part_type"},
		"other sub-object":  {func(in *schema.ECPartInput) { in.Rings = &schema.ECRings{} }, "rings"},
		"unknown size": {func(in *schema.ECPartInput) {
			in.PartType, in.Domes, in.Sliders = "sliders", nil, &schema.ECSliders{Size: new("3U")}
		}, "sliders.size"},
		"unapproved vendor": {func(in *schema.ECPartInput) { in.Purchase = &schema.ECPartPurchaseInput{Vendor: new("Nowhere")} }, "purchase.vendor"},
	} {
		in := validECPartInput()
		tc.edit(&in)

		_, _, err := handleCreateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateECPartInput{ECPartInput: in})

		s.ErrorContains(err, tc.want, name)
	}
}

func (s *HandleCreateECPartSuite) TestRepositoryError_ReturnsError() {
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.New("dynamodb down"))

	_, _, err := handleCreateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateECPartInput{ECPartInput: validECPartInput()})

	s.ErrorContains(err, "failed to create EC part")
}

type HandleUpdateECPartSuite struct {
	suite.Suite

	mockRepo  *mocks.MockECPartRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleUpdateECPartSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateECPartSuite))
}

func (s *HandleUpdateECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleUpdateECPartSuite) TestSucceeds() {
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.mockRepo.EXPECT().
		Update(mock.Anything, mock.MatchedBy(func(p repository.ECPart) bool { return p.ID == "p1" })).
		RunAndReturn(func(_ context.Context, p repository.ECPart) (*repository.ECPart, error) { return &p, nil })

	_, out, err := handleUpdateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.UpdateECPartInput{ECPartID: "p1", ECPartInput: validECPartInput()})

	s.Require().NoError(err)
	s.Equal("p1", out.ECPart.ID)
}

func (s *HandleUpdateECPartSuite) TestBlankID_ReturnsError() {
	_, _, err := handleUpdateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.UpdateECPartInput{ECPartInput: validECPartInput()})

	s.ErrorContains(err, "ec_part_id")
}

func (s *HandleUpdateECPartSuite) TestNotFound_ReturnsMutationNotFound() {
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).Return(nil, repository.ErrNotFound)

	_, _, err := handleUpdateECPart(s.mockRepo, s.mockPrefs)(callerContext(s.T()), nil, schema.UpdateECPartInput{ECPartID: "p1", ECPartInput: validECPartInput()})

	s.ErrorIs(err, errMutationNotFound)
}

type HandleSetECPartImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockECPartRepository
	mockImages  *mocks.MockECPartImageStore
	mockUploads *mocks.MockUploadCounter
}

func TestHandleSetECPartImageSuite(t *testing.T) {
	suite.Run(t, new(HandleSetECPartImageSuite))
}

func (s *HandleSetECPartImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

func (s *HandleSetECPartImageSuite) call(in schema.SetECPartImageInput) (schema.SetECPartImageOutput, error) {
	_, out, err := handleSetECPartImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleSetECPartImageSuite) TestReplacingImage_PresignsThenDeletesPreviousThenSetsPath() {
	previous := repository.ECPartImageKey("ec-parts/" + callerID + "/p1/image/old")
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &previous}, nil)
	presigned := s.mockImages.EXPECT().PresignPut(mock.Anything, mock.Anything, "image/webp", int64(1000)).Return("https://example.com/put", nil).NotBefore(reserved)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, previous).Return(nil).NotBefore(presigned)
	s.mockRepo.EXPECT().SetImagePath(mock.Anything, "p1", mock.MatchedBy(func(k repository.ECPartImageKey) bool { return k != previous })).Return(nil).NotBefore(deleted)

	out, err := s.call(schema.SetECPartImageInput{ECPartID: "p1", ContentType: "image/webp", SizeBytes: 1000})

	s.Require().NoError(err)
	s.Equal("https://example.com/put", out.UploadURL)
}

func (s *HandleSetECPartImageSuite) TestInvalidInputs_ReturnErrors() {
	for name, in := range map[string]schema.SetECPartImageInput{
		"blank id":        {ContentType: "image/webp", SizeBytes: 1000},
		"unapproved type": {ECPartID: "p1", ContentType: "image/png", SizeBytes: 1000},
		"too large":       {ECPartID: "p1", ContentType: "image/webp", SizeBytes: 10 << 20},
	} {
		_, err := s.call(in)
		s.Error(err, name)
	}
}

func (s *HandleSetECPartImageSuite) TestPartNotFound_ReturnsMutationNotFound() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(nil, repository.ErrNotFound)

	_, err := s.call(schema.SetECPartImageInput{ECPartID: "p1", ContentType: "image/webp", SizeBytes: 1000})

	s.ErrorIs(err, errMutationNotFound)
}

type HandleDeleteECPartImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockECPartRepository
	mockImages *mocks.MockECPartImageStore
}

func TestHandleDeleteECPartImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteECPartImageSuite))
}

func (s *HandleDeleteECPartImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
}

func (s *HandleDeleteECPartImageSuite) TestDeletesObjectThenClearsPath() {
	key := repository.ECPartImageKey("ec-parts/" + callerID + "/p1/image/u1")
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(&repository.ECPart{ID: "p1", ImagePath: &key}, nil)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil).Call
	s.mockRepo.EXPECT().ClearImagePath(mock.Anything, "p1").Return(&key, nil).NotBefore(deleted)

	_, _, err := handleDeleteECPartImage(s.mockRepo, s.mockImages)(callerContext(s.T()), nil, schema.DeleteECPartImageInput{ECPartID: "p1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteECPartImageSuite) TestNoImage_SucceedsWithoutDeleting() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(&repository.ECPart{ID: "p1"}, nil)

	_, _, err := handleDeleteECPartImage(s.mockRepo, s.mockImages)(callerContext(s.T()), nil, schema.DeleteECPartImageInput{ECPartID: "p1"})

	s.Require().NoError(err)
}

type HandleDeleteECPartSuite struct {
	suite.Suite

	mockRepo        *mocks.MockECPartRepository
	mockBuilds      *mocks.MockBuildRepository
	mockBuildImages *mocks.MockBuildImageStore
	mockImages      *mocks.MockECPartImageStore
}

func TestHandleDeleteECPartSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteECPartSuite))
}

func (s *HandleDeleteECPartSuite) SetupTest() {
	s.mockRepo = mocks.NewMockECPartRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockECPartImageStore(s.T())
}

func (s *HandleDeleteECPartSuite) call(in schema.DeleteECPartInput) (schema.DeleteECPartOutput, error) {
	_, out, err := handleDeleteECPart(s.mockRepo, s.mockBuilds, s.mockBuildImages, s.mockImages)(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleDeleteECPartSuite) TestDefaultBlock_Unreferenced_Deletes() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, callerID, "p1").Return(nil, nil)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "p1").Return(nil)

	out, err := s.call(schema.DeleteECPartInput{ECPartID: "p1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteECPartSuite) TestBlocked_ReturnsErrorListingBuilds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, callerID, "p1").Return([]string{"b1", "b2"}, nil)

	_, err := s.call(schema.DeleteECPartInput{ECPartID: "p1", OnDelete: "block"})

	s.EqualError(err, "EC part is still used by builds: b1, b2")
}

func (s *HandleDeleteECPartSuite) TestCascade_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().FindBuildsReferencingECPart(mock.Anything, callerID, "p1").Return([]string{"b1"}, nil)
	s.mockBuilds.EXPECT().Get(mock.Anything, callerID, "b1").Return(&repository.Build{ID: "b1"}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, "b1").Return(nil)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "p1").Return(nil, repository.ErrNotFound)

	out, err := s.call(schema.DeleteECPartInput{ECPartID: "p1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"b1"}, out.DeletedBuildIDs)
}

func (s *HandleDeleteECPartSuite) TestInvalidInputs_ReturnErrors() {
	_, err := s.call(schema.DeleteECPartInput{ECPartID: " "})
	s.Require().ErrorContains(err, "ec_part_id")

	_, err = s.call(schema.DeleteECPartInput{ECPartID: "p1", OnDelete: "detach"})
	s.ErrorContains(err, "on_delete")
}
