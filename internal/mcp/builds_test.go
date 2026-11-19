package mcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

func validBuildInput() schema.BuildInput {
	return schema.BuildInput{
		Keyboard:   "kb-1",
		Visibility: "private",
	}
}

type HandleCreateBuildSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockSwitches  *mocks.MockSwitchRepository
	mockECParts   *mocks.MockECPartRepository
	mockKeycaps   *mocks.MockKeycapSetRepository
}

func TestHandleCreateBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateBuildSuite))
}

func (s *HandleCreateBuildSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockECParts = mocks.NewMockECPartRepository(s.T())
	s.mockKeycaps = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateBuildSuite) handler() mcp.ToolHandlerFor[schema.CreateBuildInput, schema.CreateBuildOutput] {
	return handleCreateBuild(s.mockBuilds, s.mockKeyboards, s.mockSwitches, s.mockECParts, s.mockKeycaps, s.mockPrefs)
}

// stubOwnedKeyboard arranges keyboardRepo.Get to report "kb-1" as existing
// and owned by the test caller - the default reference-validation outcome
// most tests below want, so the handler under test can proceed to the
// behavior they're actually asserting on.
func (s *HandleCreateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil).
		Maybe()
}

func (s *HandleCreateBuildSuite) TestSucceeds() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Build.Keyboard)
	s.NotEmpty(out.Build.ID, "create must assign a server-generated id")
}

func (s *HandleCreateBuildSuite) TestBlankKeyboard_ReturnsError() {
	in := validBuildInput()
	in.Keyboard = "   "

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keyboard must not be blank")
}

func (s *HandleCreateBuildSuite) TestInvalidVisibility_ReturnsError() {
	in := validBuildInput()
	in.Visibility = "everyone"

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateBuildSuite) TestDurometerWithoutDurometerSupport_ReturnsError() {
	in := validBuildInput()
	mountType, durometer := "Top Mount", "40A"
	in.CaseMountType = &schema.BuildCaseMountType{Type: &mountType, Durometer: &durometer}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "case_mount_type.durometer")
	s.Require().ErrorContains(err, "supports durometer")
}

func (s *HandleCreateBuildSuite) TestMalformedBuildDate_ReturnsError() {
	in := validBuildInput()
	badDate := "not-a-date"
	in.BuildDate = &badDate

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "build_date")
}

func (s *HandleCreateBuildSuite) TestNonPositiveSwitchCount_ReturnsError() {
	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 0}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "switches[0].count")
}

func (s *HandleCreateBuildSuite) TestUnapprovedStabsName_ReturnsError() {
	in := validBuildInput()
	in.Stabs = &schema.BuildStabsInput{Name: strPtrMCP("NotApproved")}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "stabs.name")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestUnapprovedCaseMountType_ReturnsError() {
	in := validBuildInput()
	in.CaseMountType = &schema.BuildCaseMountType{Type: strPtrMCP("NotApproved")}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "case_mount_type.type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorIs(err, errBuildAlreadyExists)
}

func (s *HandleCreateBuildSuite) TestMutationConflict_ReturnsMutationConflictError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleCreateBuildSuite) TestRepositoryError_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("create failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "failed to create build")
}

func (s *HandleCreateBuildSuite) TestMissingKeyboard_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "keyboard")
}

func (s *HandleCreateBuildSuite) TestMissingSwitch_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(nil, repository.ErrNotFound)

	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 4}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "switches[0].switch")
}

func (s *HandleCreateBuildSuite) TestMissingKeycapSet_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, repository.ErrNotFound)

	in := validBuildInput()
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keycap_kits[0].keycap_set")
}

func (s *HandleCreateBuildSuite) TestKeycapSetFoundButKitMissing_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"other-kit": {KitID: "other-kit"}}}, nil)

	in := validBuildInput()
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keycap_kits[0].kit")
}

func (s *HandleCreateBuildSuite) TestValidReferences_Succeeds() {
	s.stubOwnedKeyboard()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1"}}}, nil)
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 4}}
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().NoError(err)
	s.NotEmpty(out.Build.ID)
}

func (s *HandleCreateBuildSuite) TestReferenceCheckRepositoryError_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "failed to validate build")
}

type HandleUpdateBuildSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockSwitches  *mocks.MockSwitchRepository
	mockECParts   *mocks.MockECPartRepository
	mockKeycaps   *mocks.MockKeycapSetRepository
}

func TestHandleUpdateBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateBuildSuite))
}

func (s *HandleUpdateBuildSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockECParts = mocks.NewMockECPartRepository(s.T())
	s.mockKeycaps = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateBuildSuite) handler() mcp.ToolHandlerFor[schema.UpdateBuildInput, schema.UpdateBuildOutput] {
	return handleUpdateBuild(s.mockBuilds, s.mockKeyboards, s.mockSwitches, s.mockECParts, s.mockKeycaps, s.mockPrefs)
}

func (s *HandleUpdateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil).
		Maybe()
}

func (s *HandleUpdateBuildSuite) TestSucceeds() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().NoError(err)
	s.Equal("b-1", out.Build.ID, "update must target the requested id")
}

func (s *HandleUpdateBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "  ",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleUpdateBuildSuite) TestBlankKeyboard_ReturnsError() {
	in := validBuildInput()
	in.Keyboard = "   "

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{BuildID: "b-1", BuildInput: in})

	s.Require().ErrorContains(err, "keyboard must not be blank")
}

func (s *HandleUpdateBuildSuite) TestMissingKeyboard_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorContains(err, "keyboard")
}

func (s *HandleUpdateBuildSuite) TestNotFound_ReturnsNotFound() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "missing",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleUpdateBuildSuite) TestMutationConflict_ReturnsConflictError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorIs(err, errMutationConflict)
}

func (s *HandleUpdateBuildSuite) TestRepositoryError_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("update failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

func strPtrMCP(s string) *string { return &s }

type HandleListBuildsSuite struct {
	suite.Suite

	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockSwitches  *mocks.MockSwitchRepository
	mockECParts   *mocks.MockECPartRepository
	mockKeycaps   *mocks.MockKeycapSetRepository
	mockPrefs     *mocks.MockPreferencesReader
}

func TestHandleListBuildsSuite(t *testing.T) {
	suite.Run(t, new(HandleListBuildsSuite))
}

func (s *HandleListBuildsSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockECParts = mocks.NewMockECPartRepository(s.T())
	s.mockKeycaps = mocks.NewMockKeycapSetRepository(s.T())
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleListBuildsSuite) handler() mcp.ToolHandlerFor[schema.ListBuildsInput, schema.ListBuildsOutput] {
	s.mockPrefs.EXPECT().
		GetPreferences(mock.Anything, mock.Anything).
		Return(repository.ProfilePreferences{}, nil).
		Maybe()

	return handleListBuilds(s.mockBuilds, s.mockKeyboards, s.mockSwitches, s.mockECParts, s.mockKeycaps, s.mockPrefs)
}

func (s *HandleListBuildsSuite) TestEmpty_ReturnsEmptyList() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Empty(out.Builds)
}

func (s *HandleListBuildsSuite) TestSingleBuild_ResolvableKeyboard_DenormalizesBrandAndName() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{{UserID: callerID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Brand: "Keychron", Name: "Q1"}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Builds, 1)
	s.Require().NotNil(out.Builds[0].Keyboard)
	s.Equal("Keychron", out.Builds[0].Keyboard.Brand)
	s.Equal("Q1", out.Builds[0].Keyboard.Name)
}

func (s *HandleListBuildsSuite) TestKeyboardRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{{UserID: callerID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().ErrorContains(err, "failed to list builds")
}

func (s *HandleListBuildsSuite) TestPassesLimitAndCursor() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 5, "abc").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{Limit: 5, Cursor: "abc"})

	s.Require().NoError(err)
}

func (s *HandleListBuildsSuite) TestOtherUserID_ListsThatUsersCollection() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListBuildsSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().ErrorContains(err, "failed to list builds")
}

func (s *HandleListBuildsSuite) TestInvalidCursor_ReturnsError() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "stale").
		Return(nil, "", repository.ErrInvalidCursor)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{Cursor: "stale"})

	s.Require().Error(err)
}

type HandleGetBuildSuite struct {
	suite.Suite

	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockPrefs     *mocks.MockPreferencesReader
}

func TestHandleGetBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleGetBuildSuite))
}

func (s *HandleGetBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{
			ID:     "kb-1",
			Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{{ID: "plate-1", Material: "FR4"}}),
			PCBs:   repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "pcb-1"}}),
		}, nil).
		Maybe()
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
}

func (s *HandleGetBuildSuite) handler() mcp.ToolHandlerFor[schema.GetBuildInput, schema.GetBuildOutput] {
	return handleGetBuild(s.mockBuilds, s.mockKeyboards, s.mockPrefs)
}

func (s *HandleGetBuildSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{
			ID:         "build-1",
			Keyboard:   "kb-1",
			Visibility: repository.VisibilityPrivate,
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
	s.Equal("build-1", out.Build.ID)
	s.Equal("kb-1", out.Build.Keyboard)
	s.Require().NotNil(out.Build.Visibility)
	s.Equal("private", *out.Build.Visibility)
}

func (s *HandleGetBuildSuite) TestPartsTheKeyboardStillHas_AreKept() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{
			ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate,
			Plate: new("plate-1"), PCB: new("pcb-1"),
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	_, out, err := s.handler()(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
	s.Equal(new("plate-1"), out.Build.Plate)
	s.Equal(new("pcb-1"), out.Build.PCB)
}

func (s *HandleGetBuildSuite) TestPartsTheKeyboardNoLongerHas_AreLeftOut() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{
			ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate,
			Plate: new("removed-plate"), PCB: new("removed-pcb"),
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)

	_, out, err := s.handler()(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
	s.Nil(out.Build.Plate)
	s.Nil(out.Build.PCB)
}

func (s *HandleGetBuildSuite) TestKeyboardError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{ID: "build-1", Keyboard: "kb-gone", Visibility: repository.VisibilityPrivate}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.DefaultProfilePreferences(), nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-gone").
		Return(nil, errors.New("dynamo down"))

	_, _, err := s.handler()(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().ErrorContains(err, "failed to get build")
}

func (s *HandleGetBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "  "})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleGetBuildSuite) TestNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "missing"})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleGetBuildSuite) TestOtherUsersPrivateBuild_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{ID: "build-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleGetBuildSuite) TestOtherUsersSharedVisibilityBuild_Succeeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityAuthenticated}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("build-1", out.Build.ID)
	s.Nil(out.Build.Visibility)
}

func (s *HandleGetBuildSuite) TestOtherUsersPublicBuildShowPriceToOthersTrue_IncludesStabsPrice() {
	price := 12.5
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{
			ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPublic,
			Stabs: &repository.BuildStabs{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: true}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().NoError(err)
	s.Require().NotNil(out.Build.Stabs)
	s.Require().NotNil(out.Build.Stabs.Price)
	s.InDelta(price, *out.Build.Stabs.Price, 0.0001)
}

func (s *HandleGetBuildSuite) TestOtherUsersPublicBuildShowPriceToOthersFalse_OmitsStabsPrice() {
	price := 12.5
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{
			ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPublic,
			Stabs: &repository.BuildStabs{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{ShowPriceToOthers: false}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().NoError(err)
	s.Nil(out.Build.Stabs, "stabs had only a price, so nothing is left")
}

func (s *HandleGetBuildSuite) TestOwner_AlwaysIncludesStabsPriceAndCurrency() {
	price := 12.5
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{
			ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate,
			Stabs: &repository.BuildStabs{Price: &price},
		}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
	s.Require().NotNil(out.Build.Stabs)
	s.Require().NotNil(out.Build.Stabs.Price)
	s.InDelta(price, *out.Build.Stabs.Price, 0.0001)
	s.Require().NotNil(out.Build.Stabs.Currency)
	s.Equal("EUR", *out.Build.Stabs.Currency)
}

func (s *HandleGetBuildSuite) TestOtherUsersPreferencesError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPublic}, nil)
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, otherID).Return(repository.ProfilePreferences{}, errors.New("dynamo down"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().Error(err)
}

type HandleDeleteBuildSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
	mockImages *mocks.MockBuildImageStore
}

func TestHandleDeleteBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteBuildSuite))
}

func (s *HandleDeleteBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteBuildSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, "build-1").Return(nil)

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: ""})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleDeleteBuildSuite) TestNotFound_StillSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "missing"})

	s.Require().NoError(err, "delete is idempotent: a nonexistent id is not an error")
}

func (s *HandleDeleteBuildSuite) TestImagesAreDeletedFromS3BeforeDB() {
	key1 := repository.BuildImageKey("builds/u/build-1/images/img-1")
	key2 := repository.BuildImageKey("builds/u/build-1/images/img-2")
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: key1},
			{ImageID: "img-2", Path: key2},
		})}, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key1).Return(nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key2).Return(nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, "build-1").Return(nil)

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildSuite) TestImageDeleteFails_ReturnsError_DoesNotDeleteBuild() {
	key1 := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: key1},
		})}, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key1).Return(errors.New("s3 unavailable"))

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().Error(err)
	// mockBuilds has no .EXPECT() for Delete - verifies the DB record was
	// never touched.
}

func (s *HandleDeleteBuildSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, mock.Anything).Return(errors.New("delete failed"))

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().ErrorContains(err, "failed to delete build")
}

func (s *HandleDeleteBuildSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, mock.Anything).Return(repository.ErrMutationConflict)

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().ErrorIs(err, errMutationConflict)
}

type HandleAddBuildImageSuite struct {
	suite.Suite

	mockBuilds  *mocks.MockBuildRepository
	mockImages  *mocks.MockBuildImageStore
	mockUploads *mocks.MockUploadCounter
}

func TestHandleAddBuildImageSuite(t *testing.T) {
	suite.Run(t, new(HandleAddBuildImageSuite))
}

func (s *HandleAddBuildImageSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
	s.mockUploads = mocks.NewMockUploadCounter(s.T())
}

func (s *HandleAddBuildImageSuite) TestSucceeds() {
	reserved := expectUploadReserved(s.mockUploads)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.MatchedBy(func(img repository.BuildImage) bool {
			return img.ImageID != ""
		})).
		Return(nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil).
		NotBefore(reserved)

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, out, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().NoError(err)
	s.NotEmpty(out.ImageID)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleAddBuildImageSuite) TestBlankBuildID_ReturnsError() {
	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     " ",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleAddBuildImageSuite) TestImageLimitReached_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.Anything).
		Return(fmt.Errorf("adding image: %w", repository.ErrImageLimitReached))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errImageLimitReached)
}

func (s *HandleAddBuildImageSuite) TestSizeOverCap_ReturnsError() {
	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   repository.MaxImageSizeBytes + 1,
	})

	s.Require().ErrorContains(err, "size_bytes")
}

func (s *HandleAddBuildImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "application/exe",
	})

	s.Require().ErrorContains(err, "content_type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleAddBuildImageSuite) TestBuildNotFound_ReturnsNotFound() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "missing", mock.Anything).
		Return(repository.ErrNotFound)

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "missing",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleAddBuildImageSuite) TestPresignError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("", errors.New("s3: access denied"))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorContains(err, "failed to add build image")
	// mockBuilds has no .EXPECT() for AddImage - verifies the DB was
	// never touched when presigning fails.
}

func (s *HandleAddBuildImageSuite) TestRepositoryError_ReturnsError() {
	expectUploadReserved(s.mockUploads)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/webp", int64(524288)).
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.Anything).
		Return(errors.New("put item failed"))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

type HandleDeleteBuildImageSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
	mockImages *mocks.MockBuildImageStore
}

func TestHandleDeleteBuildImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteBuildImageSuite))
}

func (s *HandleDeleteBuildImageSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteBuildImageSuite) TestSucceeds() {
	key := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key).Return(nil)
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(&key, nil)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildImageSuite) TestBlankBuildID_ReturnsError() {
	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: " ", ImageID: "img-1"})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleDeleteBuildImageSuite) TestBlankImageID_ReturnsError() {
	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: " "})

	s.Require().ErrorContains(err, "image_id must not be blank")
}

func (s *HandleDeleteBuildImageSuite) TestDeleteImageNotFound_IdempotentNoError() {
	key := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key).Return(nil)
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(nil, repository.ErrNotFound)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildImageSuite) TestAlreadyAbsent_StillSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().NoError(err, "deleting an already-absent image is not an error")
}

func (s *HandleDeleteBuildImageSuite) TestBuildNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "missing", ImageID: "img-1"})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleDeleteBuildImageSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(nil, errors.New("get failed"))

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleDeleteBuildImageSuite) TestImageDeleteFailure_ReturnsError_DoesNotDeleteDBRecord() {
	key := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: key},
		})}, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().ErrorContains(err, "failed to delete build image")
	// mockBuilds has no .EXPECT() for DeleteImage - verifies the DB record
	// was never touched.
}

func (s *HandleListBuildsSuite) TestOwnCollection_IncludesVisibility() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{
			{UserID: callerID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate},
		}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Builds, 1)
	s.Require().NotNil(out.Builds[0].Visibility)
	s.Equal("private", *out.Builds[0].Visibility)
}

func (s *HandleListBuildsSuite) TestOtherUsersCollection_OmitsVisibility() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, otherID, mock.Anything, mock.Anything, 20, "").
		Return([]repository.Build{
			{UserID: otherID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPublic},
		}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, otherID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{UserID: otherID})

	s.Require().NoError(err)
	s.Require().Len(out.Builds, 1)
	s.Nil(out.Builds[0].Visibility)
}

func (s *HandleCreateBuildSuite) TestReturnsOwnersCurrencyWithPrice() {
	s.stubOwnedKeyboard()
	price := 99.5
	s.mockBuilds.EXPECT().Create(mock.Anything, mock.Anything).
		Return(&repository.Build{ID: "b-1", Keyboard: "kb-1", Stabs: &repository.BuildStabs{Price: &price}}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Build.Stabs.Currency)
	s.Equal("EUR", *out.Build.Stabs.Currency)
}

func (s *HandleCreateBuildSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.stubOwnedKeyboard()
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().EqualError(err, "failed to create build")
}

func (s *HandleUpdateBuildSuite) TestReturnsOwnersCurrencyWithPrice() {
	s.stubOwnedKeyboard()
	price := 99.5
	s.mockBuilds.EXPECT().Update(mock.Anything, mock.Anything).
		Return(&repository.Build{ID: "b-1", Keyboard: "kb-1", Stabs: &repository.BuildStabs{Price: &price}}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{BuildID: "b-1", BuildInput: validBuildInput()})

	s.Require().NoError(err)
	s.Require().NotNil(out.Build.Stabs.Currency)
	s.Equal("EUR", *out.Build.Stabs.Currency)
}

func (s *HandleUpdateBuildSuite) TestPreferencesError_ReturnsErrorBeforeWrite() {
	s.stubOwnedKeyboard()
	s.mockPrefs.ExpectedCalls = nil
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).Return(repository.ProfilePreferences{}, errors.New("boom"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{BuildID: "b-1", BuildInput: validBuildInput()})

	s.Require().EqualError(err, "failed to update build")
}

func (s *HandleAddBuildImageSuite) TestUploadLimitReached_ReturnsUploadsUnavailable() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(repository.ErrUploadLimitReached)

	_, _, err := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errUploadsUnavailable)
}

func (s *HandleAddBuildImageSuite) TestReserveError_ReturnsFailed() {
	s.mockUploads.EXPECT().
		Increment(mock.Anything, mock.Anything, int64(500), mock.Anything).
		Return(errors.New("dynamo: boom"))

	_, _, err := handleAddBuildImage(s.mockBuilds, s.mockImages, uploadlimit.New(s.mockUploads, 500))(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/webp",
		SizeBytes:   524288,
	})

	s.Require().ErrorIs(err, errMutationFailed)
}

func (s *HandleCreateBuildSuite) TestECBuild_ValidatesPartsAndStoresThem() {
	s.stubOwnedKeyboard()
	s.mockECParts.EXPECT().Get(mock.Anything, callerID, "p-1").Return(&repository.ECPart{ID: "p-1"}, nil)
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.MatchedBy(func(b repository.Build) bool {
			return b.BuildType == "EC" && len(b.ECParts) == 1 && b.ECParts[0].ECPart == "p-1"
		})).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) { return &b, nil })

	in := validBuildInput()
	in.BuildType = "EC"
	in.ECParts = []schema.BuildECPartEntry{{ECPart: "p-1", Count: 60}}

	_, out, err := s.handler()(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().NoError(err)
	s.Equal("EC", out.Build.BuildType)
}

func (s *HandleCreateBuildSuite) TestInvalidECPartInputs_ReturnErrors() {
	for name, tc := range map[string]struct {
		edit func(*schema.BuildInput)
		want string
	}{
		"zero count": {func(in *schema.BuildInput) {
			in.BuildType, in.ECParts = "EC", []schema.BuildECPartEntry{{ECPart: "p-1"}}
		}, "ec_parts[0].count"},
		"ec parts on MX": {func(in *schema.BuildInput) { in.ECParts = []schema.BuildECPartEntry{{ECPart: "p-1", Count: 1}} }, "ec_parts"},
		"switches on EC": {func(in *schema.BuildInput) {
			in.BuildType, in.Switches = "EC", []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 1}}
		}, "switches"},
		"unknown build type": {func(in *schema.BuildInput) { in.BuildType = "Topre" }, "build_type"},
	} {
		in := validBuildInput()
		tc.edit(&in)

		_, _, err := s.handler()(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

		s.ErrorContains(err, tc.want, name)
	}
}

func (s *HandleCreateBuildSuite) TestMissingECPart_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockECParts.EXPECT().Get(mock.Anything, callerID, "p-1").Return(nil, repository.ErrNotFound)

	in := validBuildInput()
	in.BuildType = "EC"
	in.ECParts = []schema.BuildECPartEntry{{ECPart: "p-1", Count: 60}}

	_, _, err := s.handler()(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.ErrorContains(err, "ec_parts[0].ec_part")
}

func (s *HandleCreateBuildSuite) TestBlankReferenceIDs_ReturnFieldErrors() {
	for want, edit := range map[string]func(*schema.BuildInput){
		"switches[0].switch must not be blank": func(in *schema.BuildInput) { in.Switches = []schema.BuildSwitchEntry{{Switch: " ", Count: 1}} },
		"ec_parts[0].ec_part must not be blank": func(in *schema.BuildInput) {
			in.BuildType, in.ECParts = "EC", []schema.BuildECPartEntry{{ECPart: "", Count: 1}}
		},
	} {
		in := validBuildInput()
		edit(&in)

		_, _, err := s.handler()(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

		s.EqualError(err, want)
	}
}
