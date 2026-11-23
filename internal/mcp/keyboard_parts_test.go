package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type HandleKeyboardPlateSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleKeyboardPlateSuite(t *testing.T) {
	suite.Run(t, new(HandleKeyboardPlateSuite))
}

func (s *HandleKeyboardPlateSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleKeyboardPlateSuite) TestCreate_AssignsIDAndReturnsPlate() {
	s.mockKeyboards.EXPECT().
		AddPlate(mock.Anything, "kb-1", mock.MatchedBy(func(p repository.KeyboardPlate) bool {
			return p.ID != "" && p.Material == "PC"
		})).
		RunAndReturn(func(_ context.Context, _ string, p repository.KeyboardPlate) (*repository.KeyboardPlate, error) {
			return &p, nil
		})

	_, out, err := handleCreateKeyboardPlate(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateKeyboardPlateInput{
		KeyboardID:         "kb-1",
		KeyboardPlateInput: schema.KeyboardPlateInput{Material: "PC", Purchase: &schema.KeyboardPartPurchaseInput{Price: new(35.0)}},
	})

	s.Require().NoError(err)
	s.NotEmpty(out.Plate.ID)
	s.Equal("PC", out.Plate.Material)
	s.Require().NotNil(out.Plate.Purchase)
	s.Equal(new(35.0), out.Plate.Purchase.Price)
	s.Equal(new("EUR"), out.Plate.Purchase.Currency)
}

func (s *HandleKeyboardPlateSuite) TestCreate_UnapprovedMaterial_ReturnsError() {
	_, _, err := handleCreateKeyboardPlate(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateKeyboardPlateInput{
		KeyboardID:         "kb-1",
		KeyboardPlateInput: schema.KeyboardPlateInput{Material: "NotAMaterial"},
	})

	s.Require().ErrorContains(err, "material")
}

func (s *HandleKeyboardPlateSuite) TestCreate_KeyboardMissing_ReturnsNotFound() {
	s.mockKeyboards.EXPECT().AddPlate(mock.Anything, "kb-1", mock.Anything).Return(nil, repository.ErrNotFound)

	_, _, err := handleCreateKeyboardPlate(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateKeyboardPlateInput{
		KeyboardID:         "kb-1",
		KeyboardPlateInput: schema.KeyboardPlateInput{Material: "AL"},
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}

func (s *HandleKeyboardPlateSuite) TestUpdate_UsesPlateIDFromInput() {
	s.mockKeyboards.EXPECT().
		UpdatePlate(mock.Anything, "kb-1", mock.MatchedBy(func(p repository.KeyboardPlate) bool { return p.ID == "plate-1" })).
		RunAndReturn(func(_ context.Context, _ string, p repository.KeyboardPlate) (*repository.KeyboardPlate, error) {
			return &p, nil
		})

	_, out, err := handleUpdateKeyboardPlate(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.UpdateKeyboardPlateInput{
		KeyboardID:         "kb-1",
		PlateID:            "plate-1",
		KeyboardPlateInput: schema.KeyboardPlateInput{Material: "AL", Color: new("Black")},
	})

	s.Require().NoError(err)
	s.Equal("plate-1", out.Plate.ID)
	s.Equal(new("Black"), out.Plate.Color)
}

type HandleDeleteKeyboardPartSuite struct {
	suite.Suite

	mockKeyboards *mocks.MockKeyboardRepository
	mockBuilds    *mocks.MockBuildRepository
	mockBuildImg  *mocks.MockBuildImageStore
}

func TestHandleDeleteKeyboardPartSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteKeyboardPartSuite))
}

func (s *HandleDeleteKeyboardPartSuite) SetupTest() {
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImg = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteKeyboardPartSuite) TestPlateUnused_Deletes() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeyboard(mock.Anything, callerID, "kb-1").Return(nil, nil)
	s.mockKeyboards.EXPECT().DeletePlate(mock.Anything, "kb-1", "plate-1").Return(nil)

	_, out, err := handleDeleteKeyboardPlate(s.mockKeyboards, s.mockBuilds, s.mockBuildImg)(callerContext(s.T()), nil,
		schema.DeleteKeyboardPlateInput{KeyboardID: "kb-1", PlateID: "plate-1"})

	s.Require().NoError(err)
	s.Empty(out.DeletedBuildIDs)
}

func (s *HandleDeleteKeyboardPartSuite) TestPlateUsed_Block_ReturnsErrorListingBuild() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeyboard(mock.Anything, callerID, "kb-1").Return([]string{"b1"}, nil)
	s.mockBuilds.EXPECT().Get(mock.Anything, callerID, "b1").Return(&repository.Build{ID: "b1", Plate: new("plate-1")}, nil)

	_, _, err := handleDeleteKeyboardPlate(s.mockKeyboards, s.mockBuilds, s.mockBuildImg)(callerContext(s.T()), nil,
		schema.DeleteKeyboardPlateInput{KeyboardID: "kb-1", PlateID: "plate-1"})

	s.Require().ErrorContains(err, "still used by builds: b1")
}

func (s *HandleDeleteKeyboardPartSuite) TestPCBUsed_Cascade_DeletesBuild() {
	s.mockBuilds.EXPECT().FindBuildsReferencingKeyboard(mock.Anything, callerID, "kb-1").Return([]string{"b1"}, nil)
	s.mockBuilds.EXPECT().Get(mock.Anything, callerID, "b1").Return(&repository.Build{ID: "b1", PCB: new("pcb-1")}, nil)
	s.mockBuilds.EXPECT().Delete(mock.Anything, "b1").Return(nil)
	s.mockKeyboards.EXPECT().DeletePCB(mock.Anything, "kb-1", "pcb-1").Return(nil)

	_, out, err := handleDeleteKeyboardPCB(s.mockKeyboards, s.mockBuilds, s.mockBuildImg)(callerContext(s.T()), nil,
		schema.DeleteKeyboardPCBInput{KeyboardID: "kb-1", PCBID: "pcb-1", OnDelete: "cascade"})

	s.Require().NoError(err)
	s.Equal([]string{"b1"}, out.DeletedBuildIDs)
}

type HandleKeyboardPCBSuite struct {
	suite.Suite

	mockPrefs     *mocks.MockPreferencesReader
	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleKeyboardPCBSuite(t *testing.T) {
	suite.Run(t, new(HandleKeyboardPCBSuite))
}

func (s *HandleKeyboardPCBSuite) SetupTest() {
	s.mockPrefs = mocks.NewMockPreferencesReader(s.T())
	s.mockPrefs.EXPECT().GetPreferences(mock.Anything, callerID).
		Return(repository.ProfilePreferences{Currency: "EUR"}, nil).Maybe()
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleKeyboardPCBSuite) TestCreate_AssignsIDAndReturnsPCB() {
	s.mockKeyboards.EXPECT().
		AddPCB(mock.Anything, "kb-1", mock.MatchedBy(func(p repository.KeyboardPCB) bool { return p.ID != "" })).
		RunAndReturn(func(_ context.Context, _ string, p repository.KeyboardPCB) (*repository.KeyboardPCB, error) {
			return &p, nil
		})

	_, out, err := handleCreateKeyboardPCB(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateKeyboardPCBInput{
		KeyboardID:       "kb-1",
		KeyboardPCBInput: schema.KeyboardPCBInput{Assembly: new("Hotswap"), Connectivity: new("Wired")},
	})

	s.Require().NoError(err)
	s.NotEmpty(out.PCB.ID)
	s.Equal(new("Hotswap"), out.PCB.Assembly)
}

func (s *HandleKeyboardPCBSuite) TestCreate_UnapprovedValues_ReturnsErrorNamingEach() {
	bad := "NotAValue"
	_, _, err := handleCreateKeyboardPCB(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.CreateKeyboardPCBInput{
		KeyboardID:       "kb-1",
		KeyboardPCBInput: schema.KeyboardPCBInput{Firmware: &bad, Connectivity: &bad},
	})

	s.Require().ErrorContains(err, "firmware")
	s.Require().ErrorContains(err, "connectivity")
}

func (s *HandleKeyboardPCBSuite) TestUpdate_PCBMissing_ReturnsNotFound() {
	s.mockKeyboards.EXPECT().UpdatePCB(mock.Anything, "kb-1", mock.Anything).Return(nil, repository.ErrNotFound)

	_, _, err := handleUpdateKeyboardPCB(s.mockKeyboards, s.mockPrefs)(callerContext(s.T()), nil, schema.UpdateKeyboardPCBInput{
		KeyboardID: "kb-1",
		PCBID:      "nope",
	})

	s.Require().ErrorIs(err, errMutationNotFound)
}
