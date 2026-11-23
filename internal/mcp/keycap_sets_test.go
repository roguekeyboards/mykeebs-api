package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

type HandleListKeycapSetsSuite struct {
	suite.Suite

	mockRepo  *mocks.MockKeycapSetRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleListKeycapSetsSuite(t *testing.T) {
	suite.Run(t, new(HandleListKeycapSetsSuite))
}

func (s *HandleListKeycapSetsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleListKeycapSetsSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.KeycapSet{{ID: "ks-1", Brand: "GMK", Name: "Olivia"}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Equal("ks-1", out.KeycapSets[0].ID)
}

func (s *HandleListKeycapSetsSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
}

func (s *HandleListKeycapSetsSuite) TestOwnCollectionShowPriceToMeTrue_IncludesTotalCost() {
	price := 120.00
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.KeycapSet{{
			ID:   "ks-1",
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: true}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Require().NotNil(out.KeycapSets[0].TotalCost)
	s.InDelta(price, *out.KeycapSets[0].TotalCost, 0.0001)
}

func (s *HandleListKeycapSetsSuite) TestOwnCollectionShowPriceToMeFalse_OmitsTotalCost() {
	price := 120.00
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.KeycapSet{{
			ID:   "ks-1",
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: false}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Nil(out.KeycapSets[0].TotalCost)
}

func (s *HandleListKeycapSetsSuite) TestOtherUsersCollectionShowPriceToOthersFalse_OmitsTotalCost() {
	price := 120.00
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.KeycapSet{{
			ID:   "ks-1",
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Nil(out.KeycapSets[0].TotalCost)
}

func (s *HandleListKeycapSetsSuite) TestOtherUsersCollectionShowPriceToOthersTrue_IncludesTotalCost() {
	price := 120.00
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.KeycapSet{{
			ID:   "ks-1",
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSets, 1)
	s.Require().NotNil(out.KeycapSets[0].TotalCost)
	s.InDelta(price, *out.KeycapSets[0].TotalCost, 0.0001)
}

func (s *HandleListKeycapSetsSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListKeycapSetsSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, nil)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListKeycapSetsSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(s.T().Context(), nil, schema.ListKeycapSetsInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListKeycapSetsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().ErrorContains(err, "failed to list keycap sets")
}

func (s *HandleListKeycapSetsSuite) TestInvalidCursor_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{Cursor: "stale"})

	s.Require().Error(err)
}

func (s *HandleListKeycapSetsSuite) TestPreferencesError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, errors.New("dynamo down"))

	handler := handleListKeycapSets(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListKeycapSetsInput{})

	s.Require().ErrorContains(err, "failed to list keycap sets")
}

type HandleGetKeycapSetSuite struct {
	suite.Suite

	mockRepo  *mocks.MockKeycapSetRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleGetKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleGetKeycapSetSuite))
}

func (s *HandleGetKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleGetKeycapSetSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Brand:      "GMK",
			Name:       "Olivia",
			Visibility: repository.VisibilityPrivate,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID)
	s.Equal("GMK", out.KeycapSet.Brand)
	s.Require().NotNil(out.KeycapSet.Visibility)
	s.Equal("private", *out.KeycapSet.Visibility)
}

func (s *HandleGetKeycapSetSuite) TestKitsMapWithHasImage() {
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)
	imagePath := repository.KeycapKitImageKey("keycap-sets/caller-0001/ks-1/kits/kit-1/image/up1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Visibility: repository.VisibilityPrivate,
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Name: "Base", ImagePath: &imagePath},
				"kit-2": {KitID: "kit-2", Name: "Novelties"}},
		}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSet.Kits, 2)
	s.True(out.KeycapSet.Kits[0].HasImage)
	s.False(out.KeycapSet.Kits[1].HasImage)
}

func (s *HandleGetKeycapSetSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "missing"})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPrivateKeycapSet_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().ErrorIs(err, errKeycapSetNotFound)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPublicKeycapSet_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{
			ID:         "ks-1",
			Brand:      "GMK",
			Visibility: repository.VisibilityPublic,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID)
	s.Nil(out.KeycapSet.Visibility)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPublicKeycapSetShowPriceToOthersTrue_IncludesKitPrice() {
	price := 120.00
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{
			ID: "ks-1", Visibility: repository.VisibilityPublic,
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSet.Kits, 1)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase.Price)
	s.InDelta(price, *out.KeycapSet.Kits[0].Purchase.Price, 0.0001)
}

func (s *HandleGetKeycapSetSuite) TestOtherUsersPublicKeycapSetShowPriceToOthersFalse_OmitsKitPrice() {
	price := 120.00
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "ks-1").
		Return(&repository.KeycapSet{
			ID: "ks-1", Visibility: repository.VisibilityPublic,
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSet.Kits, 1)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase)
	s.Nil(out.KeycapSet.Kits[0].Purchase.Price)
}

func (s *HandleGetKeycapSetSuite) TestOwner_AlwaysIncludesKitPriceAndCurrency() {
	price := 120.00
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(&repository.KeycapSet{
			ID: "ks-1", Visibility: repository.VisibilityPrivate,
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}, nil)

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Require().Len(out.KeycapSet.Kits, 1)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase.Price)
	s.InDelta(price, *out.KeycapSet.Kits[0].Purchase.Price, 0.0001)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase.Currency)
	s.Equal("EUR", *out.KeycapSet.Kits[0].Purchase.Currency)
}

func (s *HandleGetKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetKeycapSet(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().ErrorContains(err, "failed to get keycap set")
}

func validKeycapSetInput() schema.KeycapSetInput {
	return schema.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Olivia",
		Visibility: "private",
	}
}

type HandleCreateKeycapSetSuite struct {
	suite.Suite

	mockPrefs      *mocks.MockPreferencesReader
	mockKeycapSets *mocks.MockKeycapSetRepository
}

func TestHandleCreateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeycapSetSuite))
}

func (s *HandleCreateKeycapSetSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateKeycapSetSuite) TestSucceeds() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
			return &ks, nil
		})

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().NoError(err)
	s.Equal("GMK", out.KeycapSet.Brand)
	s.NotEmpty(out.KeycapSet.ID, "create must assign a server-generated id")
}

func (s *HandleCreateKeycapSetSuite) TestUnapprovedProfile_ReturnsError() {
	profile := "NotAProfile"
	in := validKeycapSetInput()
	in.Profile = &profile

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "profile")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapSetSuite) TestUnapprovedMaterial_ReturnsError() {
	material := "NotAMaterial"
	in := validKeycapSetInput()
	in.Material = &material

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: in})

	s.Require().ErrorContains(err, "material")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapSetSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().ErrorIs(err, errKeycapSetAlreadyExists)
}

func (s *HandleCreateKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockKeycapSets.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("create failed"))

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().ErrorContains(err, "failed to create keycap set")
}

type HandleUpdateKeycapSetSuite struct {
	suite.Suite

	mockPrefs      *mocks.MockPreferencesReader
	mockKeycapSets *mocks.MockKeycapSetRepository
}

func TestHandleUpdateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeycapSetSuite))
}

func (s *HandleUpdateKeycapSetSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeycapSets = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateKeycapSetSuite) TestSucceeds() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
			return &ks, nil
		})

	handler := handleUpdateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "ks-1",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().NoError(err)
	s.Equal("ks-1", out.KeycapSet.ID, "update must target the requested id")
}

func (s *HandleUpdateKeycapSetSuite) TestNotFound_ReturnsNotFound() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "missing",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockKeycapSets.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("update failed"))

	handler := handleUpdateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{
		KeycapSetID:    "ks-1",
		KeycapSetInput: validKeycapSetInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteKeycapSetSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapSetSuite))
}

func (s *HandleDeleteKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapSetSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapSetSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeycapSetSuite) TestNotFound_StillSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "missing").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "missing"})

	s.Require().NoError(err, "delete is idempotent: a nonexistent id is not an error")
}

func (s *HandleDeleteKeycapSetSuite) TestKitImagesAreDeletedFromS3BeforeDB() {
	key1 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	key2 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-2/image/up1")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key1},
			"kit-2": {KitID: "kit-2", ImagePath: &key2}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key1).Return(nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key2).Return(nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapSetSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("delete failed"))

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().ErrorContains(err, "failed to delete keycap set")
}

func (s *HandleDeleteKeycapSetSuite) TestKitImageDeleteFails_ReturnsError_DoesNotDeleteSet() {
	key1 := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key1}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key1).Return(errors.New("s3 unavailable"))

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1"})

	s.Require().Error(err)
	// mockRepo has no .EXPECT() for Delete - verifies the DB record was
	// never touched.
}

func (s *HandleDeleteKeycapSetSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeycapSetSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapSet(mock.Anything, mock.Anything, "ks-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)
	s.mockRepo.EXPECT().Delete(mock.Anything, "ks-1").Return(nil)

	handler := handleDeleteKeycapSet(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapSetInput{KeycapSetID: "ks-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func validKeycapKitInput() schema.KeycapKitInput {
	return schema.KeycapKitInput{Name: "Base"}
}

type HandleCreateKeycapKitSuite struct {
	suite.Suite

	mockPrefs *mocks.MockPreferencesReader
	mockRepo  *mocks.MockKeycapSetRepository
}

func TestHandleCreateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateKeycapKitSuite))
}

func (s *HandleCreateKeycapKitSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateKeycapKitSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
	s.Equal("Base", out.KeycapKit.Name)
	s.NotEmpty(out.KeycapKit.KitID, "create must assign a server-generated id")
}

func (s *HandleCreateKeycapKitSuite) TestPrimaryTrue_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && *primary
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	in := validKeycapKitInput()
	primary := true
	in.Primary = &primary

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().NoError(err)
}

func (s *HandleCreateKeycapKitSuite) TestPrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
}

func (s *HandleCreateKeycapKitSuite) TestUnapprovedVendor_ReturnsError() {
	vendor := "NotARealVendor"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchaseInput{Vendor: &vendor}

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.vendor")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapKitSuite) TestUnapprovedOrderStatus_ReturnsError() {
	status := "Bogus"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchaseInput{OrderStatus: &status}

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.order_status")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateKeycapKitSuite) TestKeycapSetNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "missing", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "missing",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleCreateKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, errors.New("add kit failed"))

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleUpdateKeycapKitSuite struct {
	suite.Suite

	mockPrefs *mocks.MockPreferencesReader
	mockRepo  *mocks.MockKeycapSetRepository
}

func TestHandleUpdateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateKeycapKitSuite))
}

func (s *HandleUpdateKeycapKitSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateKeycapKitSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
	s.Equal("kit-1", out.KeycapKit.KitID, "update must target the requested id")
}

func (s *HandleUpdateKeycapKitSuite) TestPrimaryFalse_PassesPrimaryToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary != nil && !*primary
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	in := validKeycapKitInput()
	primary := false
	in.Primary = &primary

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: in,
	})

	s.Require().NoError(err)
}

func (s *HandleUpdateKeycapKitSuite) TestPrimaryOmitted_PassesNilToRepo() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.MatchedBy(func(primary *bool) bool {
			return primary == nil
		})).
		RunAndReturn(func(_ context.Context, _ string, kit repository.KeycapKit, _ *bool) (*repository.KeycapKit, error) {
			return &kit, nil
		})

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().NoError(err)
}

func (s *HandleUpdateKeycapKitSuite) TestUnapprovedVendor_ReturnsError() {
	vendor := "NotARealVendor"
	in := validKeycapKitInput()
	in.Purchase = &schema.KeycapKitPurchaseInput{Vendor: &vendor}

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: in,
	})

	s.Require().ErrorContains(err, "purchase.vendor")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleUpdateKeycapKitSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "missing-kit",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(nil, errors.New("update kit failed"))

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{
		KeycapSetID:    "ks-1",
		KitID:          "kit-1",
		KeycapKitInput: validKeycapKitInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteKeycapKitSuite struct {
	suite.Suite

	mockRepo     *mocks.MockKeycapSetRepository
	mockBuilds   *mocks.MockBuildRepository
	mockBuildImg *mocks.MockBuildImageStore
	mockImages   *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapKitSuite))
}

func (s *HandleDeleteKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapKitSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeycapKitSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteKeycapKitSuite) TestKitIDNotInSet_StillSucceeds() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "missing-kit").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().NoError(err, "delete is idempotent: a kit id not present in the set is not an error")
}

func (s *HandleDeleteKeycapKitSuite) TestKeycapSetNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "missing", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "missing", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitSuite) TestImageIsDeletedFromS3BeforeDB() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapKitSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteKit() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().Error(err)
	// mockRepo has no .EXPECT() for DeleteKit - verifies the DB record was
	// never touched.
}

func (s *HandleDeleteKeycapKitSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").Return(nil, nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(errors.New("delete kit failed"))

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteKeycapKitSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteKeycapKitSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingKeycapKit(mock.Anything, mock.Anything, "ks-1", "kit-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}}}, nil)
	s.mockRepo.EXPECT().DeleteKit(mock.Anything, "ks-1", "kit-1").Return(nil)

	handler := handleDeleteKeycapKit(s.mockRepo, s.mockBuilds, s.mockBuildImg, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

type HandleSetKeycapKitImageSuite struct {
	suite.Suite

	mockRepo    *mocks.MockKeycapSetRepository
	mockImages  *mocks.MockKeycapKitImageStore
	mockUploads *mocks.MockUploadCounter
}

func TestHandleSetKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(HandleSetKeycapKitImageSuite))
}

func (s *HandleSetKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

// previousKitImageKey is kit-1's image key before a replacement.
func previousKitImageKey() repository.KeycapKitImageKey {
	return repository.KeycapKitImageKey("keycap-sets/" + callerID + "/ks-1/kits/kit-1/image/old-upload")
}

// newKitImageKey matches a freshly minted per-upload key for kit-1, which
// must never be the previous image's key.
func newKitImageKey() any {
	return mock.MatchedBy(func(k repository.KeycapKitImageKey) bool {
		return strings.HasPrefix(string(k), "keycap-sets/"+callerID+"/ks-1/kits/kit-1/image/") && k != previousKitImageKey()
	})
}

// setWithKit returns set ks-1 holding kit-1, with imagePath as its image.
func setWithKit(imagePath *repository.KeycapKitImageKey) *repository.KeycapSet {
	return &repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {ImagePath: imagePath}}}
}

func (s *HandleSetKeycapKitImageSuite) call(kitID string) (schema.SetKeycapKitImageOutput, error) {
	_, out, err := handleSetKeycapKitImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       kitID,
		ContentType: "image/webp",
		SizeBytes:   524288,
	})
	return out, err
}

func (s *HandleSetKeycapKitImageSuite) TestSucceeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(setWithKit(nil), nil)
	var presigned repository.KeycapKitImageKey
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKitImageKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.KeycapKitImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/upload", nil).
		NotBefore(reserved)
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "kit-1", newKitImageKey()).
		Run(func(_ context.Context, _, _ string, key repository.KeycapKitImageKey) { s.Equal(presigned, key) }).
		Return(nil)

	out, err := s.call("kit-1")

	s.Require().NoError(err)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleSetKeycapKitImageSuite) TestReplacingImage_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousKitImageKey()
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(setWithKit(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKitImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	deleted := s.mockImages.EXPECT().
		Delete(mock.Anything, previous).
		Return(nil).
		Call
	s.mockRepo.EXPECT().
		SetKitImagePath(mock.Anything, "ks-1", "kit-1", newKitImageKey()).
		Return(nil).
		NotBefore(deleted)

	_, err := s.call("kit-1")

	s.Require().NoError(err)
}

func (s *HandleSetKeycapKitImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleSetKeycapKitImage(s.mockRepo, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.SetKeycapKitImageInput{
		KeycapSetID: "ks-1",
		KitID:       "kit-1",
		ContentType: "application/exe",
	})

	s.Require().ErrorContains(err, "content_type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleSetKeycapKitImageSuite) TestSetNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(nil, repository.ErrNotFound)

	_, err := s.call("kit-1")

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleSetKeycapKitImageSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(setWithKit(nil), nil)

	_, err := s.call("missing-kit")

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleSetKeycapKitImageSuite) TestPresignError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	previous := previousKitImageKey()
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(setWithKit(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKitImageKey(), "image/webp", int64(524288)).
		Return("", errors.New("presign failed"))

	_, err := s.call("kit-1")

	s.Require().ErrorContains(err, "failed to set kit image")
	// No .EXPECT() for Delete or SetKitImagePath - verifies neither the
	// previous object nor the DB was touched when presigning fails.
}

func (s *HandleSetKeycapKitImageSuite) TestDeletePreviousError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	previous := previousKitImageKey()
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "ks-1").
		Return(setWithKit(&previous), nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newKitImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, previous).
		Return(errors.New("s3: access denied"))

	_, err := s.call("kit-1")

	s.Require().ErrorContains(err, "failed to set kit image")
	// No .EXPECT() for SetKitImagePath - the pointer stays on the previous
	// image, which still exists.
}

type HandleDeleteKeycapKitImageSuite struct {
	suite.Suite

	mockRepo   *mocks.MockKeycapSetRepository
	mockImages *mocks.MockKeycapKitImageStore
}

func TestHandleDeleteKeycapKitImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeycapKitImageSuite))
}

func (s *HandleDeleteKeycapKitImageSuite) SetupTest() {
	s.mockRepo = mocks.NewMockKeycapSetRepository(s.T())
	s.mockImages = mocks.NewMockKeycapKitImageStore(s.T())
}

func (s *HandleDeleteKeycapKitImageSuite) TestSucceeds() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().ClearKitImagePath(mock.Anything, "ks-1", "kit-1").Return(&key, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapKitImageSuite) TestClearKitImagePathNotFound_IdempotentNoError() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockRepo.EXPECT().ClearKitImagePath(mock.Anything, "ks-1", "kit-1").Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteKeycapKitImageSuite) TestAlreadyCleared_StillSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}}}, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().NoError(err, "clearing an already-unset image is not an error")
}

func (s *HandleDeleteKeycapKitImageSuite) TestKitNotInSet_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1"}, nil)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitImageSuite) TestKitNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "missing-kit"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteKeycapKitImageSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, errors.New("get failed"))

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteKeycapKitImageSuite) TestImageDeleteFailure_ReturnsError_DoesNotClearDBRecord() {
	key := repository.KeycapKitImageKey("keycap-sets/u/ks-1/kits/kit-1/image/up1")
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", ImagePath: &key}}}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteKeycapKitImage(s.mockRepo, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteKeycapKitImageInput{KeycapSetID: "ks-1", KitID: "kit-1"})

	s.Require().ErrorContains(err, "failed to delete kit image")
	// mockRepo has no .EXPECT() for ClearKitImagePath - verifies the DB
	// record was never touched.
}

func (s *HandleCreateKeycapSetSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeycapSets.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}}}, nil)

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase.Currency)
	s.Equal("EUR", *out.KeycapSet.Kits[0].Purchase.Currency)
}

func (s *HandleCreateKeycapSetSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleCreateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapSetInput{KeycapSetInput: validKeycapSetInput()})

	s.Require().EqualError(err, "failed to create keycap set")
}

func (s *HandleUpdateKeycapSetSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockKeycapSets.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &price}}}}, nil)

	handler := handleUpdateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{KeycapSetID: "ks-1", KeycapSetInput: validKeycapSetInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.KeycapSet.Kits[0].Purchase.Currency)
	s.Equal("EUR", *out.KeycapSet.Kits[0].Purchase.Currency)
}

func (s *HandleUpdateKeycapSetSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleUpdateKeycapSet(s.mockKeycapSets, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapSetInput{KeycapSetID: "ks-1", KeycapSetInput: validKeycapSetInput()})

	s.Require().EqualError(err, "failed to update keycap set")
}

func (s *HandleCreateKeycapKitSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockRepo.EXPECT().AddKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}}, nil)

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{KeycapSetID: "ks-1", KeycapKitInput: validKeycapKitInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.KeycapKit.Purchase.Currency)
	s.Equal("EUR", *out.KeycapKit.Purchase.Currency)
}

func (s *HandleCreateKeycapKitSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleCreateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateKeycapKitInput{KeycapSetID: "ks-1", KeycapKitInput: validKeycapKitInput()})

	s.Require().EqualError(err, "failed to create keycap kit")
}

func (s *HandleUpdateKeycapKitSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockRepo.EXPECT().UpdateKit(mock.Anything, "ks-1", mock.Anything, mock.Anything).
		Return(&repository.KeycapKit{KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}}, nil)

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", KeycapKitInput: validKeycapKitInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.KeycapKit.Purchase.Currency)
	s.Equal("EUR", *out.KeycapKit.Purchase.Currency)
}

func (s *HandleUpdateKeycapKitSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleUpdateKeycapKit(s.mockRepo, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateKeycapKitInput{KeycapSetID: "ks-1", KitID: "kit-1", KeycapKitInput: validKeycapKitInput()})

	s.Require().EqualError(err, "failed to update keycap kit")
}

func (s *HandleSetKeycapKitImageSuite) TestUploadLimitReached_ReturnsUploadsUnavailable() {
	previous := previousKitImageKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "ks-1").Return(setWithKit(&previous), nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	_, err := s.call("kit-1")

	s.Require().ErrorIs(err, errUploadsUnavailable)
}

func (s *HandleSetKeycapKitImageSuite) TestReserveError_ReturnsFailed() {
	previous := previousKitImageKey()
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "ks-1").Return(setWithKit(&previous), nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	_, err := s.call("kit-1")

	s.Require().ErrorIs(err, errMutationFailed)
}
