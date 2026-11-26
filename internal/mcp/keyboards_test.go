package mcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type HandleListKeyboardsSuite struct {
	suite.Suite

	mockRepo  *mocks.MockKeyboardRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleListKeyboardsSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeyboardsSuite))
}

func (s *HandleListKeyboardsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleListKeyboardsSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.Keyboard{{ID: "kb-1", Brand: "Mode", Name: "Sixty"}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Equal("kb-1", out.Keyboards[0].ID)
}

func (s *HandleListKeyboardsSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestOwnCollectionShowPriceToMeTrue_IncludesPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Keyboard{{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: true}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Require().NotNil(out.Keyboards[0].TotalCost)
	s.InDelta(price, *out.Keyboards[0].TotalCost, 0.0001)
}

func (s *HandleListKeyboardsSuite) TestOwnCollectionShowPriceToMeFalse_OmitsPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Keyboard{{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: false}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Nil(out.Keyboards[0].TotalCost)
}

func (s *HandleListKeyboardsSuite) TestOtherUsersCollectionShowPriceToOthersFalse_OmitsPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Keyboard{{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Nil(out.Keyboards[0].TotalCost)
}

func (s *HandleListKeyboardsSuite) TestOtherUsersCollectionShowPriceToOthersTrue_IncludesPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Keyboard{{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Keyboards, 1)
	s.Require().NotNil(out.Keyboards[0].TotalCost)
	s.InDelta(price, *out.Keyboards[0].TotalCost, 0.0001)
}

func (s *HandleListKeyboardsSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListKeyboardsSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListKeyboardsSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(s.T().Context(), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListKeyboardsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorContains(err, "failed to list keyboards")
}

func (s *HandleListKeyboardsSuite) TestInvalidCursor_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{Cursor: "stale"})

	s.Require().Error(err)
}

func (s *HandleListKeyboardsSuite) TestPreferencesError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, errors.New("dynamo down"))

	handler := handleListKeyboards(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardsInput{})

	s.Require().ErrorContains(err, "failed to list keyboards")
}

type HandleGetKeyboardSuite struct {
	suite.Suite

	mockRepo  *mocks.MockKeyboardRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleGetKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeyboardSuite))
}

func (s *HandleGetKeyboardSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleGetKeyboardSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Name:       "Sixty",
			Visibility: repository.VisibilityPrivate,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
	s.Equal("Mode", out.Keyboard.Brand)
	s.Require().NotNil(out.Keyboard.Visibility)
	s.Equal("private", *out.Keyboard.Visibility)
}

func (s *HandleGetKeyboardSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "missing"})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPrivateKeyboard_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPublicKeyboard_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Brand:      "Mode",
			Visibility: repository.VisibilityPublic,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID)
	s.Nil(out.Keyboard.Visibility)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPublicKeyboardShowPriceToOthersTrue_IncludesPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{
			ID: "kb-1", Visibility: repository.VisibilityPublic,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().NotNil(out.Keyboard.Purchase)
	s.Require().NotNil(out.Keyboard.Purchase.Price)
	s.InDelta(price, *out.Keyboard.Purchase.Price, 0.0001)
}

func (s *HandleGetKeyboardSuite) TestOtherUsersPublicKeyboardShowPriceToOthersFalse_OmitsPrice() {
	price := 199.99
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{
			ID: "kb-1", Visibility: repository.VisibilityPublic,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().NotNil(out.Keyboard.Purchase)
	s.Nil(out.Keyboard.Purchase.Price)
}

func (s *HandleGetKeyboardSuite) TestOwner_AlwaysIncludesPriceAndCurrency() {
	price := 199.99
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID: "kb-1", Visibility: repository.VisibilityPrivate,
			Purchase: repository.KeyboardPurchase{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}, nil)

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Require().NotNil(out.Keyboard.Purchase)
	s.Require().NotNil(out.Keyboard.Purchase.Price)
	s.InDelta(price, *out.Keyboard.Purchase.Price, 0.0001)
	s.Require().NotNil(out.Keyboard.Purchase.Currency)
	s.Equal("EUR", *out.Keyboard.Purchase.Currency)
}

func (s *HandleGetKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetKeyboard(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeyboardInput{KeyboardID: "kb-1"})

	s.Require().ErrorContains(err, "failed to get keyboard")
}

func validKeyboardInput() schema.KeyboardInput {
	return schema.KeyboardInput{
		Brand:      "Mode",
		Name:       "Sixty",
		Visibility: "private",
	}
}

type HandleCreateKeyboardSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleCreateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeyboardSuite))
}

func (s *HandleCreateKeyboardSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleCreateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().NoError(err)
	s.Equal("Mode", out.Keyboard.Brand)
	s.NotEmpty(out.Keyboard.ID, "create must assign a server-generated id")
}

func (s *HandleCreateKeyboardSuite) TestUnapprovedSize_ReturnsError() {
	size := "NotASize"
	in := validKeyboardInput()
	in.Size = &size

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().ErrorContains(err, "size")
	s.Require().ErrorContains(err, "not an approved")
}

// The cross-field rule: both values are individually approved, but the
// layout doesn't list this size among the ones it supports.
func (s *HandleCreateKeyboardSuite) TestLayoutNotValidForSize_ReturnsError() {
	size := "60%"
	layout := "MIT"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	// Both values are individually approved, so the generic wording would
	// claim the layout isn't an approved *size* - sending an agent to fix a
	// field that's already correct.
	s.Require().ErrorContains(err, `"MIT" is not a valid layout for size "60%"`)
	s.Require().NotContains(err.Error(), "approved keyboard_size")
}

func (s *HandleCreateKeyboardSuite) TestLayoutValidForSize_Succeeds() {
	size := "60%"
	layout := "WKL"
	in := validKeyboardInput()
	in.Size = &size
	in.Layout = &layout

	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().NoError(err)
}

func (s *HandleCreateKeyboardSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().ErrorIs(err, errKeyboardAlreadyExists)
}

type HandleUpdateKeyboardSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleUpdateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeyboardSuite))
}

func (s *HandleUpdateKeyboardSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleUpdateKeyboardSuite) TestSucceeds() {
	s.mockKeyboards.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "kb-1",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Keyboard.ID, "update must target the requested id")
}

func (s *HandleUpdateKeyboardSuite) TestNotFound_ReturnsNotFound() {
	s.mockKeyboards.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{
		KeyboardID:    "kb-1",
		KeyboardInput: validKeyboardInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

type HandleDeleteKeyboardSuite struct {
	suite.Suite

	mockKeyboards      *mocks.MockKeyboardRepository
	mockBuilds         *mocks.MockBuildRepository
	mockBuildImages    *mocks.MockBuildImageStore
	mockKeyboardImages *mocks.MockKeyboardImageStore
}

func TestHandleDeleteKeyboardSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardSuite))
}

func (s *HandleDeleteKeyboardSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockKeyboardImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleDeleteKeyboardSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeyboardSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(nil)

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)
	s.mockKeyboards.EXPECT().
		Delete(mock.Anything, "kb-1").
		Return(errors.New("delete failed"))

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeyboardSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteKeyboard() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeyboard(mock.Anything, mock.Anything, "kb-1").
		Return(nil, nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img-1", Path: "keyboards/alice/kb-1/images/img-1"},
		})}, nil)
	s.mockKeyboardImages.EXPECT().
		DeleteKeyboardImage(mock.Anything, repository.KeyboardImageKey("keyboards/alice/kb-1/images/img-1")).
		Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeyboard(s.mockKeyboards, s.mockBuilds, s.mockBuildImages, s.mockKeyboardImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardInput{KeyboardID: "kb-1"})

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() for Delete - verifies the DB record
	// was never touched.
}

func (s *HandleCreateKeyboardSuite) TestWellFormedDates_Succeed() {
	ordered, delivered := "2026-01-15", "2026-02-01"
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchaseInput{OrderDate: &ordered, DeliveryDate: &delivered}

	s.mockKeyboards.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, kb repository.Keyboard) (*repository.Keyboard, error) {
			return &kb, nil
		})

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().NoError(err)
}

func (s *HandleCreateKeyboardSuite) TestEstimatedDeliveryDateBeforeOrderDate_ReturnsErrorWithoutCreating() {
	in := validKeyboardInput()
	in.Purchase = &schema.KeyboardPurchaseInput{OrderDate: new("2026-03-01"), EstimatedDeliveryDate: new("2026-02-28")}

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: in})

	s.Require().EqualError(err, "purchase.estimated_delivery_date: can't be before order_date")
}

type HandleListKeyboardImagesSuite struct {
	suite.Suite

	mockRepo *mocks.MockKeyboardRepository
}

func TestHandleListKeyboardImagesSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeyboardImagesSuite))
}

func (s *HandleListKeyboardImagesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleListKeyboardImagesSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{
			ID:         "kb-1",
			Visibility: repository.VisibilityPrivate,
			Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
				{ImageID: "img-1", Path: "keyboards/caller-0001/kb-1/images/img-1"},
				{ImageID: "img-2", Path: "keyboards/caller-0001/kb-1/images/img-2"},
			}),
		}, nil)

	handler := handleListKeyboardImages(s.mockRepo)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeyboardImagesInput{KeyboardID: "kb-1"})

	s.Require().NoError(err)
	s.Equal([]schema.KeyboardImage{{ImageID: "img-1"}, {ImageID: "img-2"}}, out.Images)
}

func (s *HandleListKeyboardImagesSuite) TestKeyboardNotFound_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := handleListKeyboardImages(s.mockRepo)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeyboardImagesInput{KeyboardID: "kb-1"})

	s.Require().ErrorIs(err, errKeyboardNotFound)
}

type HandleAddKeyboardImageSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockImages    *mocks.MockKeyboardImageStore
	mockUploads   *mocks.MockUploadCounter
}

func TestHandleAddKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(HandleAddKeyboardImageSuite))
}

func (s *HandleAddKeyboardImageSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

func (s *HandleAddKeyboardImageSuite) TestSucceeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.MatchedBy(func(img repository.KeyboardImage) bool {
			return img.ImageID != ""
		})).
		Return(nil)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil).
		NotBefore(reserved)

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, out, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().NoError(err)
	s.NotEmpty(out.ImageID)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleAddKeyboardImageSuite) TestImageLimitReached_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.Anything).
		Return(fmt.Errorf("adding image: %w", repository.ErrImageLimitReached))

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errImageLimitReached)
}

func (s *HandleAddKeyboardImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "application/pdf",
	})

	s.Require().ErrorContains(err, "content_type")
}

func (s *HandleAddKeyboardImageSuite) TestNotFound_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockKeyboards.EXPECT().
		AddImage(mock.Anything, "kb-1", mock.Anything).
		Return(repository.ErrNotFound)

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleAddKeyboardImageSuite) TestPresignError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutKeyboardImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	handler := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() for AddImage - verifies the DB was
	// never touched when presigning fails.
}

type HandleDeleteKeyboardImageSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockImages    *mocks.MockKeyboardImageStore
}

func TestHandleDeleteKeyboardImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardImageSuite))
}

func (s *HandleDeleteKeyboardImageSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockImages = mocks.NewMockKeyboardImageStore(s.T())
}

func (s *HandleDeleteKeyboardImageSuite) TestSucceeds() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(nil)
	s.mockKeyboards.EXPECT().DeleteImage(mock.Anything, "kb-1", "img-1").Return(&key, nil)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardImageSuite) TestDeleteImageNotFound_IdempotentNoError() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(nil)
	s.mockKeyboards.EXPECT().DeleteImage(mock.Anything, "kb-1", "img-1").Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardImageSuite) TestNotFound_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeyboardImageSuite) TestAlreadyAbsent_SucceedsWithoutS3Call() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeyboardImageSuite) TestS3DeleteFails_ReturnsError_DoesNotDeleteDBRecord() {
	key := repository.KeyboardImageKey("keyboards/u/kb-1/images/img-1")
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteKeyboardImage(mock.Anything, key).Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeyboardImage(s.mockKeyboards, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeyboardImageInput{KeyboardID: "kb-1", ImageID: "img-1"})

	s.Require().Error(err)
	// mockKeyboards has no .EXPECT() for DeleteImage - verifies the DB
	// record was never touched.
}

func (s *HandleCreateKeyboardSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeyboards.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}, nil)

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Keyboard.Purchase.Currency)
	s.Equal("EUR", *out.Keyboard.Purchase.Currency)
}

func (s *HandleCreateKeyboardSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleCreateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeyboardInput{KeyboardInput: validKeyboardInput()})

	s.Require().EqualError(err, "failed to create keyboard")
}

func (s *HandleUpdateKeyboardSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeyboards.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.Keyboard{ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: &price}}, nil)

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{KeyboardID: "kb-1", KeyboardInput: validKeyboardInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Keyboard.Purchase.Currency)
	s.Equal("EUR", *out.Keyboard.Purchase.Currency)
}

func (s *HandleUpdateKeyboardSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleUpdateKeyboard(s.mockKeyboards, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeyboardInput{KeyboardID: "kb-1", KeyboardInput: validKeyboardInput()})

	s.Require().EqualError(err, "failed to update keyboard")
}

func (s *HandleAddKeyboardImageSuite) TestUploadLimitReached_ReturnsUploadsUnavailable() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	_, _, err := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errUploadsUnavailable)
}

func (s *HandleAddKeyboardImageSuite) TestReserveError_ReturnsFailed() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	_, _, err := handleAddKeyboardImage(s.mockKeyboards, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.AddKeyboardImageInput{
		KeyboardID:  "kb-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errMutationFailed)
}
