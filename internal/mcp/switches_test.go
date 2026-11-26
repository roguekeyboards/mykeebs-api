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

func deliveredSwitch(id string, quantity int) repository.Switch {
	return repository.Switch{ID: id, Visibility: repository.VisibilityPublic, Purchases: repository.BulkPurchases{
		{Quantity: new(quantity), OrderStatus: new("Delivered")},
	}}
}

var sw1On65SwitchBuild = repository.Build{Keyboard: "kb-1", Switches: []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 65}}}

type HandleListSwitchesSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	mockRepo  *mocks.MockSwitchRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleListSwitchesSuite(t *testing.T) {
	suite.Run(t, new(HandleListSwitchesSuite))
}

func (s *HandleListSwitchesSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, mock.Anything, mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
}

func (s *HandleListSwitchesSuite) TestBlankUserID_DefaultsToCaller() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, defaultListLimit, "").
		Return([]repository.Switch{{ID: "sw-1", Brand: "Gateron", Name: "Oil King", Type: "linear"}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].ID)
	s.Empty(out.NextCursor)
}

func (s *HandleListSwitchesSuite) TestSubtractsCurrentBuilds() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{deliveredSwitch("sw-1", 90)}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.builds = []repository.Build{sw1On65SwitchBuild}

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Equal(25, out.Switches[0].AvailableQuantity)
}

func (s *HandleListSwitchesSuite) TestMinAvailableQuantity_DropsSwitchesBelowIt() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{deliveredSwitch("sw-1", 90), deliveredSwitch("sw-2", 120)}, "next", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.builds = []repository.Build{sw1On65SwitchBuild}

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{MinAvailableQuantity: new(90)})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-2", out.Switches[0].ID)
	s.Equal("next", out.NextCursor)
}

func (s *HandleListSwitchesSuite) TestBuildsError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{{ID: "sw-1"}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.buildsErr = errors.New("boom")

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().EqualError(err, "failed to list switches")
}

// The caller reading their own collection may see every tier; reading
// someone else's is capped at public + authenticated.
func (s *HandleListSwitchesSuite) TestOwnCollection_ReadsAllVisibilityTiers() {
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
}

func (s *HandleListSwitchesSuite) TestOwnCollectionShowPriceToMeTrue_IncludesPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: true}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Require().NotNil(out.Switches[0].Price)
	s.InDelta(price, *out.Switches[0].Price, 0.0001)
}

func (s *HandleListSwitchesSuite) TestOwnCollectionShowPriceToMeFalse_OmitsPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{ShowPriceToMe: false}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Nil(out.Switches[0].Price)
}

func (s *HandleListSwitchesSuite) TestOtherUsersCollectionShowPriceToOthersFalse_OmitsPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Nil(out.Switches[0].Price)
}

func (s *HandleListSwitchesSuite) TestOtherUsersCollectionShowPriceToOthersTrue_IncludesPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, mock.Anything).
		Return([]repository.Switch{{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}}, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Switches, 1)
	s.Require().NotNil(out.Switches[0].Price)
	s.InDelta(price, *out.Switches[0].Price, 0.0001)
}

func (s *HandleListSwitchesSuite) TestOtherUsersCollection_ExcludesPrivate() {
	s.mockRepo.EXPECT().
		List(mock.Anything, otherID, []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
		}, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListSwitchesSuite) TestCursorIsPropagated() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "page-2").
		Return(nil, "page-3", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, nil)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{Cursor: "page-2"})

	s.Require().NoError(err)
	s.Equal("page-3", out.NextCursor)
}

func (s *HandleListSwitchesSuite) TestNoCallerIdentity_ReturnsError() {
	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(s.T().Context(), nil, schema.ListSwitchesInput{})

	s.Require().ErrorIs(err, errNoCallerIdentity)
}

func (s *HandleListSwitchesSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", errors.New("query failed"))

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().ErrorContains(err, "failed to list switches")
}

func (s *HandleListSwitchesSuite) TestInvalidCursor_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{Cursor: "stale"})

	s.Require().Error(err)
}

func (s *HandleListSwitchesSuite) TestPreferencesError_ReturnsError() {
	s.mockRepo.EXPECT().
		List(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, "", nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, mock.Anything).Return(repository.ProfilePreferences{}, errors.New("dynamo down"))

	handler := handleListSwitches(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.ListSwitchesInput{})

	s.Require().ErrorContains(err, "failed to list switches")
}

type HandleGetSwitchSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	mockRepo  *mocks.MockSwitchRepository
	mockPrefs *mocks.MockPreferencesReader
}

func TestHandleGetSwitchSuite(t *testing.T) {
	suite.Run(t, new(HandleGetSwitchSuite))
}

func (s *HandleGetSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockSwitchRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, mock.Anything, mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
}

func (s *HandleGetSwitchSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Brand:      "Gateron",
			Name:       "Oil King",
			Type:       "linear",
			Visibility: repository.VisibilityPrivate,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
	s.Equal("sw-1", out.Switch.ID)
	s.Equal("Gateron", out.Switch.Brand)
	s.Require().NotNil(out.Switch.Visibility)
	s.Equal("private", *out.Switch.Visibility)
}

func (s *HandleGetSwitchSuite) TestSubtractsCurrentBuilds() {
	sw := deliveredSwitch("sw-1", 90)
	s.mockRepo.EXPECT().Get(mock.Anything, callerID, "sw-1").Return(&sw, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, nil)
	s.builds = []repository.Build{sw1On65SwitchBuild}

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
	s.Equal(25, out.Switch.AvailableQuantity)
}

func (s *HandleGetSwitchSuite) TestNotFound_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "missing"})

	s.Require().ErrorIs(err, errSwitchNotFound)
}

// An unreadable switch must be indistinguishable from a missing one, so a
// caller can't probe for the existence of another user's private items.
func (s *HandleGetSwitchSuite) TestOtherUsersPrivateSwitch_ReturnsNotFound() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().ErrorIs(err, errSwitchNotFound)
}

func (s *HandleGetSwitchSuite) TestOtherUsersPublicSwitch_Succeeds() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID:         "sw-1",
			Brand:      "Gateron",
			Visibility: repository.VisibilityPublic,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("sw-1", out.Switch.ID)
	s.Nil(out.Switch.Visibility)
}

func (s *HandleGetSwitchSuite) TestOtherUsersPublicSwitchShowPriceToOthersTrue_IncludesPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID: "sw-1", Visibility: repository.VisibilityPublic,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Switch.Purchases, 1)
	s.Require().NotNil(out.Switch.Purchases[0].Price)
	s.InDelta(price, *out.Switch.Purchases[0].Price, 0.0001)
}

func (s *HandleGetSwitchSuite) TestOtherUsersPublicSwitchShowPriceToOthersFalse_OmitsPrice() {
	price := 8.50
	s.mockRepo.EXPECT().
		Get(mock.Anything, otherID, "sw-1").
		Return(&repository.Switch{
			ID: "sw-1", Visibility: repository.VisibilityPublic,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Switch.Purchases, 1)
	s.Nil(out.Switch.Purchases[0].Price)
}

func (s *HandleGetSwitchSuite) TestOwner_AlwaysIncludesPriceAndCurrency() {
	price := 8.50
	s.mockRepo.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{
			ID: "sw-1", Visibility: repository.VisibilityPrivate,
			Purchases: repository.BulkPurchases{{Price: &price}},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}, nil)

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
	s.Require().Len(out.Switch.Purchases, 1)
	s.Require().NotNil(out.Switch.Purchases[0].Price)
	s.InDelta(price, *out.Switch.Purchases[0].Price, 0.0001)
	s.Require().NotNil(out.Switch.Purchases[0].Currency)
	s.Equal("EUR", *out.Switch.Purchases[0].Currency)
}

func (s *HandleGetSwitchSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		Get(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("get failed"))

	handler := handleGetSwitch(s.mockRepo, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetSwitchInput{SwitchID: "sw-1"})

	s.Require().ErrorContains(err, "failed to get switch")
}

// validInput is the minimal body satisfying the required fields, for tests
// that care about something other than required-field handling.
func validInput() schema.SwitchInput {
	return schema.SwitchInput{
		Brand:      "Gateron",
		Name:       "Oil King",
		Type:       "Linear",
		Visibility: "private",
	}
}

type HandleCreateSwitchSuite struct {
	suite.Suite

	mockPrefs    *mocks.MockPreferencesReader
	mockSwitches *mocks.MockSwitchRepository
}

func TestHandleCreateSwitchSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateSwitchSuite))
}

func (s *HandleCreateSwitchSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
}

func (s *HandleCreateSwitchSuite) TestSucceeds() {
	s.mockSwitches.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, sw repository.Switch) (*repository.Switch, error) {
			return &sw, nil
		})

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: validInput()})

	s.Require().NoError(err)
	s.Equal("Gateron", out.Switch.Brand)
	s.NotEmpty(out.Switch.ID, "create must assign a server-generated id")
}

func (s *HandleCreateSwitchSuite) TestUnapprovedLookupValue_ReturnsError() {
	in := validInput()
	in.Type = "NotAType"

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: in})

	s.Require().ErrorContains(err, "not an approved")
	s.Require().ErrorContains(err, "type")
}

func (s *HandleCreateSwitchSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.mockSwitches.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: validInput()})

	s.Require().ErrorIs(err, errSwitchAlreadyExists)
}

func (s *HandleCreateSwitchSuite) TestRepositoryError_ReturnsError() {
	s.mockSwitches.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("put failed"))

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: validInput()})

	s.Require().ErrorContains(err, "failed to create switch")
}

func (s *HandleCreateSwitchSuite) TestWellFormedDates_Succeed() {
	ordered, delivered := "2026-01-15", "2026-02-01"
	in := validInput()
	in.Purchases = []schema.SwitchPurchaseInput{{OrderDate: &ordered, DeliveryDate: &delivered}}

	s.mockSwitches.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, sw repository.Switch) (*repository.Switch, error) {
			return &sw, nil
		})

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: in})

	s.Require().NoError(err)
}

type HandleUpdateSwitchSuite struct {
	suite.Suite
	mockBuilds *mocks.MockBuildRepository
	builds     []repository.Build
	buildsErr  error

	mockPrefs    *mocks.MockPreferencesReader
	mockSwitches *mocks.MockSwitchRepository
}

func TestHandleUpdateSwitchSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateSwitchSuite))
}

func (s *HandleUpdateSwitchSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.builds, s.buildsErr = nil, nil
	s.mockBuilds.EXPECT().List(mock.Anything, mock.Anything, mock.Anything, "", mock.Anything, "").
		RunAndReturn(func(context.Context, string, []repository.Visibility, string, int, string) ([]repository.Build, string, error) {
			return s.builds, "", s.buildsErr
		}).Maybe()
}

func (s *HandleUpdateSwitchSuite) TestSucceeds() {
	s.mockSwitches.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, sw repository.Switch) (*repository.Switch, error) {
			return &sw, nil
		})

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{
		SwitchID:    "sw-1",
		SwitchInput: validInput(),
	})

	s.Require().NoError(err)
	s.Equal("sw-1", out.Switch.ID, "update must target the requested id")
}

func (s *HandleUpdateSwitchSuite) TestSubtractsCurrentBuilds() {
	sw := deliveredSwitch("sw-1", 90)
	s.mockSwitches.EXPECT().Update(mock.Anything, mock.Anything).Return(&sw, nil)
	s.builds = []repository.Build{sw1On65SwitchBuild}

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{SwitchID: "sw-1", SwitchInput: validInput()})

	s.Require().NoError(err)
	s.Equal(25, out.Switch.AvailableQuantity)
}

func (s *HandleUpdateSwitchSuite) TestBuildsError_ReturnsErrorBeforeWrite() {
	s.buildsErr = errors.New("boom")

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{SwitchID: "sw-1", SwitchInput: validInput()})

	s.Require().EqualError(err, "failed to update switch")
}

func (s *HandleUpdateSwitchSuite) TestNotFound_ReturnsNotFound() {
	s.mockSwitches.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{
		SwitchID:    "missing",
		SwitchInput: validInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

type HandleDeleteSwitchSuite struct {
	suite.Suite

	mockSwitches     *mocks.MockSwitchRepository
	mockBuilds       *mocks.MockBuildRepository
	mockBuildImages  *mocks.MockBuildImageStore
	mockSwitchImages *mocks.MockSwitchImageStore
}

func TestHandleDeleteSwitchSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteSwitchSuite))
}

func (s *HandleDeleteSwitchSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockSwitchImages = mocks.NewMockSwitchImageStore(s.T())
}

func (s *HandleDeleteSwitchSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, mock.Anything, "sw-1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	s.mockSwitches.EXPECT().Delete(mock.Anything, "sw-1").Return(nil)

	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteSwitchSuite) TestInvalidOnDelete_ReturnsError() {
	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1", OnDelete: "bogus"})

	s.Require().Error(err)
}

func (s *HandleDeleteSwitchSuite) TestBlock_Referenced_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, mock.Anything, "sw-1").
		Return([]string{"build-1"}, nil)

	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1", OnDelete: "block"})

	s.Require().ErrorContains(err, "build-1")
}

func (s *HandleDeleteSwitchSuite) TestCascade_Referenced_ReturnsDeletedBuildIDs() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, mock.Anything, "sw-1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(mock.Anything, "build-1").
		Return(nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	s.mockSwitches.EXPECT().Delete(mock.Anything, "sw-1").Return(nil)

	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"build-1"}, out.DeletedBuildIDs)
}

func (s *HandleDeleteSwitchSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, mock.Anything, "sw-1").
		Return(nil, nil)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	s.mockSwitches.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("delete failed"))

	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1"})

	s.Require().ErrorContains(err, "failed to delete switch")
}

func (s *HandleDeleteSwitchSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteSwitch() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingSwitch(mock.Anything, mock.Anything, "sw-1").
		Return(nil, nil)
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &key}, nil)
	s.mockSwitchImages.EXPECT().
		Delete(mock.Anything, key).
		Return(errors.New("s3 unavailable"))

	handler := handleDeleteSwitch(s.mockSwitches, s.mockBuilds, s.mockBuildImages, s.mockSwitchImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchInput{SwitchID: "sw-1"})

	s.Require().Error(err)
	// mockSwitches has no .EXPECT() for Delete - verifies the DB record
	// was never touched.
}

type HandleSetSwitchImageSuite struct {
	suite.Suite

	mockSwitches *mocks.MockSwitchRepository
	mockImages   *mocks.MockSwitchImageStore
	mockUploads  *mocks.MockUploadCounter
}

func TestHandleSetSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(HandleSetSwitchImageSuite))
}

func (s *HandleSetSwitchImageSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

// previousSwitchImageKey is sw-1's image key before a replacement.
func previousSwitchImageKey() repository.SwitchImageKey {
	return repository.SwitchImageKey("switches/" + callerID + "/sw-1/image/old-upload")
}

// newSwitchImageKey matches a freshly minted per-upload key for sw-1, which
// must never be the previous image's key.
func newSwitchImageKey() any {
	return mock.MatchedBy(func(k repository.SwitchImageKey) bool {
		return strings.HasPrefix(string(k), "switches/"+callerID+"/sw-1/image/") && k != previousSwitchImageKey()
	})
}

func (s *HandleSetSwitchImageSuite) call() (schema.SetSwitchImageOutput, error) {
	_, out, err := handleSetSwitchImage(s.mockSwitches, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.SetSwitchImageInput{
		SwitchID:    "sw-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})
	return out, err
}

func (s *HandleSetSwitchImageSuite) TestSucceeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockSwitches.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	var presigned repository.SwitchImageKey
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Run(func(_ context.Context, key repository.SwitchImageKey, _ string, _ int64) { presigned = key }).
		Return("https://example.com/upload", nil).
		NotBefore(reserved)
	s.mockSwitches.EXPECT().
		SetImagePath(mock.Anything, "sw-1", newSwitchImageKey()).
		Run(func(_ context.Context, _ string, key repository.SwitchImageKey) { s.Equal(presigned, key) }).
		Return(nil)

	out, err := s.call()

	s.Require().NoError(err)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleSetSwitchImageSuite) TestReplacingImage_DeletesPreviousObjectFirst() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageKey()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	deleted := s.mockImages.EXPECT().
		Delete(mock.Anything, previous).
		Return(nil).
		Call
	s.mockSwitches.EXPECT().
		SetImagePath(mock.Anything, "sw-1", newSwitchImageKey()).
		Return(nil).
		NotBefore(deleted)

	_, err := s.call()

	s.Require().NoError(err)
}

func (s *HandleSetSwitchImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleSetSwitchImage(s.mockSwitches, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.SetSwitchImageInput{
		SwitchID:    "sw-1",
		ContentType: "application/x-not-an-image",
	})

	s.Require().ErrorContains(err, "content_type")
}

func (s *HandleSetSwitchImageSuite) TestNotFound_ReturnsError() {
	s.mockSwitches.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(nil, repository.ErrNotFound)

	_, err := s.call()

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleSetSwitchImageSuite) TestPresignError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageKey()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	_, err := s.call()

	s.Require().Error(err)
	// No .EXPECT() for Delete or SetImagePath - verifies neither the
	// previous object nor the DB was touched when presigning fails.
}

func (s *HandleSetSwitchImageSuite) TestDeletePreviousError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	previous := previousSwitchImageKey()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, callerID, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &previous}, nil)
	s.mockImages.EXPECT().
		PresignPut(mock.Anything, newSwitchImageKey(), "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockImages.EXPECT().
		Delete(mock.Anything, previous).
		Return(errors.New("s3: access denied"))

	_, err := s.call()

	s.Require().Error(err)
	// No .EXPECT() for SetImagePath - the pointer stays on the previous
	// image, which still exists.
}

type HandleDeleteSwitchImageSuite struct {
	suite.Suite

	mockSwitches *mocks.MockSwitchRepository
	mockImages   *mocks.MockSwitchImageStore
}

func TestHandleDeleteSwitchImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteSwitchImageSuite))
}

func (s *HandleDeleteSwitchImageSuite) SetupTest() {
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockImages = mocks.NewMockSwitchImageStore(s.T())
}

func (s *HandleDeleteSwitchImageSuite) TestSucceeds() {
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockSwitches.EXPECT().ClearImagePath(mock.Anything, "sw-1").Return(&key, nil)

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteSwitchImageSuite) TestClearImagePathNotFound_IdempotentNoError() {
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(nil)
	s.mockSwitches.EXPECT().ClearImagePath(mock.Anything, "sw-1").Return(nil, repository.ErrNotFound)

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteSwitchImageSuite) TestAlreadyCleared_StillSucceeds() {
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().NoError(err, "clearing an already-unset image is not an error")
}

func (s *HandleDeleteSwitchImageSuite) TestNotFound_ReturnsNotFound() {
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteSwitchImageSuite) TestRepositoryError_ReturnsError() {
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(nil, errors.New("get item failed"))

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteSwitchImageSuite) TestS3DeleteError_ReturnsError_DoesNotClearDBRecord() {
	key := repository.SwitchImageKey("switches/u/sw-1/image/up1")
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1", ImagePath: &key}, nil)
	s.mockImages.EXPECT().Delete(mock.Anything, key).Return(errors.New("s3: access denied"))

	handler := handleDeleteSwitchImage(s.mockSwitches, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteSwitchImageInput{SwitchID: "sw-1"})

	s.Require().ErrorContains(err, "failed to delete switch image")
	// mockSwitches has no .EXPECT() for ClearImagePath - verifies the DB
	// record was never touched.
}

func (s *HandleCreateSwitchSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockSwitches.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.Switch{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}, nil)

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: validInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Switch.Purchases[0].Currency)
	s.Equal("EUR", *out.Switch.Purchases[0].Currency)
}

func (s *HandleCreateSwitchSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleCreateSwitch(s.mockSwitches, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateSwitchInput{SwitchInput: validInput()})

	s.Require().EqualError(err, "failed to create switch")
}

func (s *HandleUpdateSwitchSuite) TestReturnsOwnersCurrencyWithPrice() {
	price := 99.5
	s.mockSwitches.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.Switch{ID: "sw-1", Purchases: repository.BulkPurchases{{Price: &price}}}, nil)

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{SwitchID: "sw-1", SwitchInput: validInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Switch.Purchases[0].Currency)
	s.Equal("EUR", *out.Switch.Purchases[0].Currency)
}

func (s *HandleUpdateSwitchSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := handleUpdateSwitch(s.mockSwitches, s.mockBuilds, s.mockPrefs)
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateSwitchInput{SwitchID: "sw-1", SwitchInput: validInput()})

	s.Require().EqualError(err, "failed to update switch")
}

func (s *HandleSetSwitchImageSuite) TestUploadLimitReached_ReturnsUploadsUnavailable() {
	previous := previousSwitchImageKey()
	s.mockSwitches.EXPECT().Get(mock.Anything, callerID, "sw-1").Return(&repository.Switch{ID: "sw-1", ImagePath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	_, err := s.call()

	s.Require().ErrorIs(err, errUploadsUnavailable)
}

func (s *HandleSetSwitchImageSuite) TestReserveError_ReturnsFailed() {
	previous := previousSwitchImageKey()
	s.mockSwitches.EXPECT().Get(mock.Anything, callerID, "sw-1").Return(&repository.Switch{ID: "sw-1", ImagePath: &previous}, nil)
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	_, err := s.call()

	s.Require().ErrorIs(err, errMutationFailed)
}
