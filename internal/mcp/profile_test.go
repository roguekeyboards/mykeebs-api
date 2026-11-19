package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	ctxpkg "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type HandleGetProfileSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleGetProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleGetProfileSuite))
}

func (s *HandleGetProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleGetProfileSuite) TestBlankIdentifier_Errors() {
	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "  "})

	s.Require().Error(err)
}

func (s *HandleGetProfileSuite) TestDiscoverableByID_Returned() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: true, Bio: sp("keebs")}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
	s.Equal("user-alice", out.Profile.UserID) // needed for the collection tools
	s.Require().NotNil(out.Profile.Bio)
	s.Equal("keebs", *out.Profile.Bio)
	s.False(out.Profile.HasAvatar)
	s.Nil(out.Profile.Preferences)
}

func (s *HandleGetProfileSuite) TestDiscoverableByUsername_Returned() {
	s.mockRepo.EXPECT().Get(mock.Anything, "alice").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "alice").Return("user-alice", nil)
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: true}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleGetProfileSuite) TestNonDiscoverable_Owner_Returned() {
	ctx := ctxpkg.WithUserID(s.T().Context(), "user-alice")
	s.mockRepo.EXPECT().Get(ctx, "user-alice").
		Return(&repository.Profile{
			OwnerID: "user-alice", Username: "alice", Discoverable: false,
			Preferences: repository.ProfilePreferences{Currency: "EUR"},
		}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, out, err := handler(ctx, nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
	s.Require().NotNil(out.Profile.Preferences)
	s.Equal("EUR", out.Profile.Preferences.Currency)
}

func (s *HandleGetProfileSuite) TestNonDiscoverable_OtherCaller_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").
		Return(&repository.Profile{OwnerID: "user-alice", Username: "alice", Discoverable: false}, nil)

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().ErrorIs(err, errProfileNotFound)
}

func (s *HandleGetProfileSuite) TestNoSuchProfile_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "ghost").Return(nil, repository.ErrNotFound)
	s.mockRepo.EXPECT().ResolveUsername(mock.Anything, "ghost").Return("", repository.ErrNotFound)

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "ghost"})

	s.Require().ErrorIs(err, errProfileNotFound)
}

func (s *HandleGetProfileSuite) TestStoreError_GenericError() {
	s.mockRepo.EXPECT().Get(mock.Anything, "user-alice").Return(nil, errors.New("dynamo down"))

	handler := handleGetProfile(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetProfileInput{Identifier: "user-alice"})

	s.Require().Error(err)
	s.NotErrorIs(err, errProfileNotFound)
}

func sp(v string) *string { return &v }

func validPreferences() *schema.ProfilePreferences {
	return &schema.ProfilePreferences{Currency: "USD", ShowPriceToMe: true, ShowPriceToOthers: false}
}

type HandleCreateProfileSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleCreateProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateProfileSuite))
}

func (s *HandleCreateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleCreateProfileSuite) call(in schema.CreateProfileInput) (schema.CreateProfileOutput, error) {
	_, out, err := handleCreateProfile(s.mockRepo)(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleCreateProfileSuite) TestValid_Created() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{OwnerID: callerID, Username: "alice"}, nil)

	out, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleCreateProfileSuite) TestInvalidUsername_ErrorNoRepoCall() {
	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "AB"}})

	s.Require().Error(err)
	s.Contains(err.Error(), "username")
}

func (s *HandleCreateProfileSuite) TestLinkHTTPURL_Error() {
	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{
		Username: "alice",
		Links:    []schema.ProfileLink{{Name: "site", URL: "http://x.example"}},
	}})

	s.Require().Error(err)
	s.Contains(err.Error(), "links[0].url")
}

func (s *HandleCreateProfileSuite) TestAlreadyExists_Error() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().ErrorIs(err, errProfileAlreadyExists)
}

func (s *HandleCreateProfileSuite) TestUsernameTaken_Error() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().Error(err)
	s.Equal(`username "alice" is already taken`, err.Error())
}

func (s *HandleCreateProfileSuite) TestMutationConflict_Error() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleCreateProfileSuite) TestRepoError_GenericError() {
	s.mockRepo.EXPECT().Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	_, err := s.call(schema.CreateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().Error(err)
	s.NotErrorIs(err, errProfileAlreadyExists)
}

type HandleUpdateProfileSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleUpdateProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateProfileSuite))
}

func (s *HandleUpdateProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleUpdateProfileSuite) call(in schema.UpdateProfileInput) (schema.UpdateProfileOutput, error) {
	_, out, err := handleUpdateProfile(s.mockRepo)(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleUpdateProfileSuite) TestValid_Updated() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.MatchedBy(func(p repository.Profile) bool {
		return p.Username == "alice"
	})).Return(&repository.Profile{OwnerID: callerID, Username: "alice"}, nil)

	out, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().NoError(err)
	s.Equal("alice", out.Profile.Username)
}

func (s *HandleUpdateProfileSuite) TestInvalidUsername_ErrorNoRepoCall() {
	_, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "AB"}})

	s.Require().Error(err)
	s.Contains(err.Error(), "username")
}

func (s *HandleUpdateProfileSuite) TestNoProfile_NotFoundError() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	_, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateProfileSuite) TestUsernameTaken_Error() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrUsernameTaken)

	_, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "taken", Preferences: validPreferences()}})

	s.Require().Error(err)
	s.Equal(`username "taken" is already taken`, err.Error())
}

func (s *HandleUpdateProfileSuite) TestMutationConflict_RetryableError() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	_, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleUpdateProfileSuite) TestRepoError_GenericError() {
	s.mockRepo.EXPECT().Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamo down"))

	_, err := s.call(schema.UpdateProfileInput{ProfileInput: schema.ProfileInput{Username: "alice", Preferences: validPreferences()}})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteProfileSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockOwnerImageStore
}

func TestHandleDeleteProfileSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteProfileSuite))
}

func (s *HandleDeleteProfileSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockOwnerImageStore(s.T())
}

// emptyItemStore is an accountdelete.ItemStore with nothing to delete.
type emptyItemStore struct{}

func (emptyItemStore) DeleteAll(context.Context) error { return nil }

func (s *HandleDeleteProfileSuite) call() error {
	account := accountdelete.Account{
		Images:     s.mockImages,
		Builds:     emptyItemStore{},
		Keyboards:  emptyItemStore{},
		Switches:   emptyItemStore{},
		ECParts:    emptyItemStore{},
		KeycapSets: emptyItemStore{},
		Profiles:   s.mockRepo,
	}
	_, _, err := handleDeleteProfile(account)(callerContext(s.T()), nil, schema.DeleteProfileInput{})
	return err
}

func (s *HandleDeleteProfileSuite) TestDeletesAccount_NoError() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything).Return(nil)

	s.Require().NoError(s.call())
}

func (s *HandleDeleteProfileSuite) TestDeleteFails_GenericError() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(errors.New("s3 down"))

	s.Require().Error(s.call())
}

func (s *HandleDeleteProfileSuite) TestMutationConflict_ConflictError() {
	s.mockImages.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything).Return(repository.ErrMutationConflict)

	s.ErrorIs(s.call(), errMutationConflict)
}

type HandleListProfilesSuite struct {
	suite.Suite

	mockRepo *mocks.MockProfileRepository
}

func TestHandleListProfilesSuite(t *testing.T) {
	suite.Run(t, new(HandleListProfilesSuite))
}

func (s *HandleListProfilesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
}

func (s *HandleListProfilesSuite) call(in schema.ListProfilesInput) (schema.ListProfilesOutput, error) {
	handler := handleListProfiles(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, in)
	return out, err
}

func (s *HandleListProfilesSuite) TestNoFilters_ReturnsSummaries() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", defaultListLimit, "").
		Return([]repository.Profile{
			{OwnerID: "user-alice", Username: "alice", Discoverable: true},
		}, "next", nil)

	out, err := s.call(schema.ListProfilesInput{})

	s.Require().NoError(err)
	s.Equal("next", out.NextCursor)
	s.Require().Len(out.Profiles, 1)
	s.Equal("alice", out.Profiles[0].Username)
	s.Equal("user-alice", out.Profiles[0].UserID)
	s.False(out.Profiles[0].HasAvatar)
}

func (s *HandleListProfilesSuite) TestUsernameFilter_Forwarded() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "al", "", defaultListLimit, "").
		Return([]repository.Profile{}, "", nil)

	_, err := s.call(schema.ListProfilesInput{Username: "al"})

	s.Require().NoError(err)
}

func (s *HandleListProfilesSuite) TestDiscordFilter_Forwarded() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "cool", defaultListLimit, "").
		Return([]repository.Profile{}, "", nil)

	_, err := s.call(schema.ListProfilesInput{DiscordUsername: "cool"})

	s.Require().NoError(err)
}

func (s *HandleListProfilesSuite) TestBothFilters_ErrorNoRepoCall() {
	_, err := s.call(schema.ListProfilesInput{Username: "al", DiscordUsername: "cool"})

	s.Require().Error(err)
}

func (s *HandleListProfilesSuite) TestClampsLimitAndPassesCursor() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", maxListLimit, "page-2").
		Return([]repository.Profile{}, "", nil)

	_, err := s.call(schema.ListProfilesInput{Limit: 5000, Cursor: "page-2"})

	s.Require().NoError(err)
}

func (s *HandleListProfilesSuite) TestRepoError_GenericError() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", defaultListLimit, "").
		Return(nil, "", errors.New("dynamo down"))

	_, err := s.call(schema.ListProfilesInput{})

	s.Require().Error(err)
}

func (s *HandleListProfilesSuite) TestInvalidCursor_Error() {
	s.mockRepo.EXPECT().
		ListPublic(mock.Anything, "", "", defaultListLimit, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	_, err := s.call(schema.ListProfilesInput{Cursor: "stale"})

	s.Require().Error(err)
}

type HandleSetProfileImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockProfileRepository
	mockImages  *mocks.MockProfileImageStore
	mockUploads *mocks.MockUploadCounter
}

func TestHandleSetProfileImageSuite(t *testing.T) {
	suite.Run(t, new(HandleSetProfileImageSuite))
}

func (s *HandleSetProfileImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

func (s *HandleSetProfileImageSuite) call(in schema.SetProfileImageInput) (schema.SetProfileImageOutput, error) {
	_, out, err := handleSetProfileImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, in)
	return out, err
}

// previousAvatarKey is the caller's avatar key before a replacement.
func previousAvatarKey() repository.ProfileImageKey {
	return repository.ProfileImageKey("profiles/" + callerID + "/avatar/old-upload")
}

// newAvatarKey matches a freshly minted per-upload avatar key, which must
// never be the previous avatar's key.
func newAvatarKey() any {
	return mock.MatchedBy(func(k repository.ProfileImageKey) bool {
		return strings.HasPrefix(string(k), "profiles/"+callerID+"/avatar/") && k != previousAvatarKey()
	})
}

func (s *HandleSetProfileImageSuite) TestValid_ReturnsUploadURL() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{}, nil)
	var presigned repository.ProfileImageKey
	s.mockImages.EXPECT().PresignPut(mock.Anything, newAvatarKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.ProfileImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/put", nil).
		NotBefore(reserved)
	s.mockRepo.EXPECT().SetAvatarPath(mock.Anything, newAvatarKey()).
		Run(func(_ context.Context, key repository.ProfileImageKey) { s.Equal(presigned, key) }).
		Return(nil)

	out, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().NoError(err)
	s.Equal("https://example.com/put", out.UploadURL)
}

func (s *HandleSetProfileImageSuite) TestReplacingAvatar_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousAvatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newAvatarKey(), "image/webp", int64(524288)).
		Return("https://example.com/put", nil)
	deleted := s.mockImages.EXPECT().Delete(mock.Anything, previous).Return(nil).Call
	s.mockRepo.EXPECT().SetAvatarPath(mock.Anything, newAvatarKey()).Return(nil).NotBefore(deleted)

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().NoError(err)
}

func (s *HandleSetProfileImageSuite) TestSizeOverCap_ErrorNoRepoCall() {
	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: repository.MaxImageSizeBytes + 1})

	s.Require().ErrorContains(err, "size_bytes")
	s.mockRepo.AssertNotCalled(s.T(), "SetAvatarPath", mock.Anything, mock.Anything)
}

func (s *HandleSetProfileImageSuite) TestUnapprovedContentType_ErrorNoRepoCall() {
	_, err := s.call(schema.SetProfileImageInput{ContentType: "application/pdf"})

	s.Require().Error(err)
	s.mockRepo.AssertNotCalled(s.T(), "SetAvatarPath", mock.Anything, mock.Anything)
}

func (s *HandleSetProfileImageSuite) TestNoProfile_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(nil, repository.ErrNotFound)

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleSetProfileImageSuite) TestMutationConflict_RetryableError() {
	expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newAvatarKey(), "image/webp", int64(524288)).
		Return("https://example.com/put", nil)
	s.mockRepo.EXPECT().SetAvatarPath(mock.Anything, newAvatarKey()).Return(repository.ErrMutationConflict)

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().Error(err)
}

func (s *HandleSetProfileImageSuite) TestPresignError_GenericError() {
	expectUploadReserved(s.mockUploads)
	previous := previousAvatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newAvatarKey(), "image/webp", int64(524288)).
		Return("", errors.New("s3 down"))

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().Error(err)
	// No .EXPECT() for Delete or SetAvatarPath - verifies neither the
	// previous object nor the DB was touched when presigning fails.
}

func (s *HandleSetProfileImageSuite) TestDeletePreviousError_GenericError() {
	expectUploadReserved(s.mockUploads)
	previous := previousAvatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockImages.EXPECT().PresignPut(mock.Anything, newAvatarKey(), "image/webp", int64(524288)).
		Return("https://example.com/put", nil)
	s.mockImages.EXPECT().Delete(mock.Anything, previous).Return(errors.New("s3 down"))

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().Error(err)
	// No .EXPECT() for SetAvatarPath - the pointer stays on the previous
	// avatar, which still exists.
}

type HandleDeleteProfileImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockProfileRepository
	mockImages *mocks.MockProfileImageStore
}

func TestHandleDeleteProfileImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteProfileImageSuite))
}

func (s *HandleDeleteProfileImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockProfileRepository(s.T())
	s.mockImages = mocks.NewMockProfileImageStore(s.T())
}

func (s *HandleDeleteProfileImageSuite) call() error {
	_, _, err := handleDeleteProfileImage(s.mockRepo, s.mockImages)(callerContext(s.T()), nil, schema.DeleteProfileImageInput{})
	return err
}

func (s *HandleDeleteProfileImageSuite) avatarKey() repository.ProfileImageKey {
	return repository.ProfileImageKey("profiles/" + callerID + "/avatar/up1")
}

func (s *HandleDeleteProfileImageSuite) TestDeletesAvatar_S3BeforeDB() {
	key := s.avatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).
		Return(&repository.Profile{OwnerID: callerID, Username: "alice", AvatarPath: &key}, nil)

	var order []string
	s.mockImages.EXPECT().Delete(mock.Anything, key).
		Run(func(context.Context, repository.ProfileImageKey) { order = append(order, "s3") }).
		Return(nil)
	s.mockRepo.EXPECT().ClearAvatarPath(mock.Anything).
		Run(func(context.Context) { order = append(order, "db") }).
		Return(&key, nil)

	s.Require().NoError(s.call())
	s.Equal([]string{"s3", "db"}, order)
}

func (s *HandleDeleteProfileImageSuite) TestClearAvatarPathNotFound_IdempotentNoError() {
	key := s.avatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).
		Return(&repository.Profile{OwnerID: callerID, Username: "alice", AvatarPath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().ClearAvatarPath(mock.Anything).Return(nil, repository.ErrNotFound)

	s.Require().NoError(s.call())
}

func (s *HandleDeleteProfileImageSuite) TestNoAvatar_IdempotentNoS3Call() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).
		Return(&repository.Profile{OwnerID: callerID, Username: "alice"}, nil)

	s.Require().NoError(s.call())
	s.mockImages.AssertNotCalled(s.T(), "Delete", mock.Anything, mock.Anything)
}

func (s *HandleDeleteProfileImageSuite) TestNoProfile_NotFoundError() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(nil, repository.ErrNotFound)

	s.Require().Error(s.call())
}

func (s *HandleDeleteProfileImageSuite) TestS3DeleteFails_ErrorNoDBClear() {
	key := s.avatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).
		Return(&repository.Profile{OwnerID: callerID, Username: "alice", AvatarPath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 down"))

	s.Require().Error(s.call())
	s.mockRepo.AssertNotCalled(s.T(), "ClearAvatarPath", mock.Anything)
}

func (s *HandleDeleteProfileImageSuite) TestGetError_GenericError() {
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(nil, errors.New("dynamo down"))

	s.Require().Error(s.call())
}

func (s *HandleSetProfileImageSuite) TestUploadLimitReached_ReturnsUploadsUnavailable() {
	previous := previousAvatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().ErrorIs(err, errUploadsUnavailable)
}

func (s *HandleSetProfileImageSuite) TestReserveError_ReturnsFailed() {
	previous := previousAvatarKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID).Return(&repository.Profile{AvatarPath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	_, err := s.call(schema.SetProfileImageInput{ContentType: "image/webp", SizeBytes: 524288})

	s.Require().ErrorIs(err, errMutationFailed)
}
