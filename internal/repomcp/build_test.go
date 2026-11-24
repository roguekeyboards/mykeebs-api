package repomcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type BuildToMCPSuite struct {
	suite.Suite
}

func TestBuildToMCPSuite(t *testing.T) {
	suite.Run(t, new(BuildToMCPSuite))
}

func strPtr(s string) *string     { return &s }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }

func (s *BuildToMCPSuite) TestMapsAllFields() {
	plate := "p-1"
	pcb := "b-1"
	notes := "first build"
	buildDate := "2026-01-15"

	out := Build{}.ToMCP(repository.Build{
		ID:       "build-1",
		Keyboard: "kb-1",
		Plate:    &plate,
		PCB:      &pcb,
		CaseMountType: &repository.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &repository.BuildStabs{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: []repository.BuildSwitchEntry{
			{Switch: "sw-1", Count: 70},
		},
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks-1", Kit: "kit-1"},
		},
		BuildDate:  &buildDate,
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
		Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img-1", Path: repository.BuildImageKey("builds/u-1/build-1/images/img-1")},
		}),
	}, true, repository.ProfilePreferences{})

	s.Equal("build-1", out.ID)
	s.Equal("kb-1", out.Keyboard)
	s.Equal(&plate, out.Plate)
	s.Equal(&pcb, out.PCB)
	s.Require().NotNil(out.CaseMountType)
	s.Equal("Top Mount", *out.CaseMountType.Type)
	s.Equal("70A", *out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal("Durock v3", *out.Stabs.Name)
	s.Require().NotNil(out.Stabs.Price)
	s.InDelta(12.5, *out.Stabs.Price, 0.0001)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].Switch)
	s.Equal(70, out.Switches[0].Count)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks-1", out.KeycapKits[0].KeycapSet)
	s.Equal("kit-1", out.KeycapKits[0].Kit)
	s.Equal(&buildDate, out.BuildDate)
	s.Require().NotNil(out.Visibility)
	s.Equal("public", *out.Visibility)
	s.True(out.HasImages)
}

func (s *BuildToMCPSuite) TestNonOwner_OmitsVisibility() {
	out := Build{}.ToMCP(repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPublic}, false, repository.ProfilePreferences{})

	s.Nil(out.Visibility)
}

func (s *BuildToMCPSuite) TestNonOwnerShowPriceToOthersFalse_OmitsStabsPrice() {
	out := Build{}.ToMCP(repository.Build{
		ID:       "build-1",
		Keyboard: "kb-1",
		Stabs: &repository.BuildStabs{
			Name:  strPtr("Durock v3"),
			Price: floatPtr(12.5),
		},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Require().NotNil(out.Stabs)
	s.Nil(out.Stabs.Price)
	s.Equal(strPtr("Durock v3"), out.Stabs.Name)
	s.Nil(out.Stabs.Currency)
}

func (s *BuildToMCPSuite) TestNonOwnerShowPriceToOthersTrue_IncludesStabsPrice() {
	out := Build{}.ToMCP(repository.Build{
		ID:       "build-1",
		Keyboard: "kb-1",
		Stabs: &repository.BuildStabs{
			Name:  strPtr("Durock v3"),
			Price: floatPtr(12.5),
		},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})

	s.Require().NotNil(out.Stabs)
	s.Require().NotNil(out.Stabs.Price)
	s.InDelta(12.5, *out.Stabs.Price, 0.0001)
	s.Require().NotNil(out.Stabs.Currency)
	s.Equal("EUR", *out.Stabs.Currency)
}

func (s *BuildToMCPSuite) TestOwner_AlwaysIncludesStabsPriceRegardlessOfShowPriceToMe() {
	out := Build{}.ToMCP(repository.Build{
		ID:       "build-1",
		Keyboard: "kb-1",
		Stabs: &repository.BuildStabs{
			Name:  strPtr("Durock v3"),
			Price: floatPtr(12.5),
		},
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})

	s.Require().NotNil(out.Stabs)
	s.Require().NotNil(out.Stabs.Price)
	s.InDelta(12.5, *out.Stabs.Price, 0.0001)
	s.Require().NotNil(out.Stabs.Currency)
	s.Equal("EUR", *out.Stabs.Currency)
}

func (s *BuildToMCPSuite) TestNoImages_HasImagesFalse() {
	out := Build{}.ToMCP(repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate}, true, repository.ProfilePreferences{})

	s.False(out.HasImages)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
}

func (s *BuildToMCPSuite) TestBuildFromMCP_MapsAllFields() {
	plate := "p-1"
	pcb := "b-1"
	notes := "first build"
	buildDate := "2026-01-15"

	out := Build{}.FromMCP(schema.BuildInput{
		Keyboard: "kb-1",
		Plate:    &plate,
		PCB:      &pcb,
		CaseMountType: &schema.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &schema.BuildStabsInput{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: []schema.BuildSwitchEntry{
			{Switch: "sw-1", Count: 70},
		},
		KeycapKits: []schema.BuildKeycapKitEntry{
			{KeycapSet: "ks-1", Kit: "kit-1"},
		},
		BuildDate:  &buildDate,
		Notes:      &notes,
		Visibility: "public",
	})

	s.Equal("kb-1", out.Keyboard)
	s.Equal(&plate, out.Plate)
	s.Equal(&pcb, out.PCB)
	s.Require().NotNil(out.CaseMountType)
	s.Equal("Top Mount", *out.CaseMountType.Type)
	s.Require().NotNil(out.Stabs)
	s.Equal("Durock v3", *out.Stabs.Name)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].Switch)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks-1", out.KeycapKits[0].KeycapSet)
	s.Equal(&buildDate, out.BuildDate)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Empty(out.ID, "ID is the caller's responsibility, not this mapping's")
	s.Nil(out.Images, "a build write never carries images")
}

func (s *BuildToMCPSuite) TestBuildFromMCP_AllOptionalFieldsNil_MapsToNil() {
	out := Build{}.FromMCP(schema.BuildInput{Keyboard: "kb-1", Visibility: "private"})

	s.Nil(out.Plate)
	s.Nil(out.PCB)
	s.Nil(out.CaseMountType)
	s.Nil(out.Stabs)
	s.Nil(out.Foam)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
	s.Nil(out.BuildDate)
	s.Nil(out.Notes)
}

type BuildToMCPSummarySuite struct {
	suite.Suite
}

func TestBuildToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(BuildToMCPSummarySuite))
}

func (s *BuildToMCPSummarySuite) TestResolvableKeyboard_DenormalizesBrandAndName() {
	buildDate := "2026-01-15"
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1", BuildDate: &buildDate}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(context.Background(), b, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Equal("build-1", out.ID)
	s.Equal("kb-1", out.KeyboardID)
	s.Equal(&buildDate, out.BuildDate)
	s.False(out.HasImage)
	s.Require().NotNil(out.Keyboard)
	s.Equal("Keychron", out.Keyboard.Brand)
	s.Equal("Q1", out.Keyboard.Name)
}

func (s *BuildToMCPSummarySuite) TestKeyboardNotFound_ReturnsError() {
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1"}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(nil, repository.ErrNotFound)

	_, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(context.Background(), b, false, repository.ProfilePreferences{})
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToMCPSummarySuite) TestKeyboardRepositoryError_ReturnsError() {
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1"}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(context.Background(), b, false, repository.ProfilePreferences{})
	s.Require().Error(err)
}

func (s *BuildToMCPSummarySuite) TestHasImages_ReportsTrue() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Images: repository.BuildImagesMap([]repository.BuildImage{{ImageID: "img-1"}}),
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	out, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(context.Background(), b, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.True(out.HasImage)
}

func (s *BuildToMCPSummarySuite) TestOwner_IncludesVisibility() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Visibility: repository.VisibilityPrivate,
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	out, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Visibility)
	s.Equal("private", *out.Visibility)
}

func (s *BuildToMCPSummarySuite) TestNonOwner_OmitsVisibility() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Visibility: repository.VisibilityPublic,
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	out, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(
		context.Background(), b, false, repository.ProfilePreferences{ShowPriceToOthers: true})
	s.Require().NoError(err)

	s.Nil(out.Visibility)
}

func (s *BuildToMCPSummarySuite) TestPriceShown_SumsComponentCosts() {
	price := 180.0
	switchPrice := 70.0
	quantity := 70
	kitPrice := 130.0
	stabsPrice := 20.0

	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Plate: strPtr("p-2"), PCB: strPtr("b-1"),
		Switches:   []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 10}},
		KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}},
		Stabs:      &repository.BuildStabs{Price: &stabsPrice},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb-1",
			Purchase: repository.KeyboardPurchase{Price: &price},
			Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{
				{ID: "p-1", Purchase: repository.KeyboardPurchase{Price: floatPtr(40)}},
				{ID: "p-2", Purchase: repository.KeyboardPurchase{Price: floatPtr(30)}},
			}),
			PCBs: repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b-1", Purchase: repository.KeyboardPurchase{Price: floatPtr(45)}}}),
		}, nil)

	switches := mocks.NewMockSwitchRepository(s.T())
	switches.EXPECT().
		Get(mock.Anything, "alice", "sw-1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw-1",
			Purchases: repository.BulkPurchases{{Price: &switchPrice, Quantity: &quantity}},
		}, nil)

	sets := mocks.NewMockKeycapSetRepository(s.T())
	sets.EXPECT().
		Get(mock.Anything, "alice", "ks-1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks-1",
			Kits: map[string]repository.KeycapKit{
				"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &kitPrice}},
			},
		}, nil)

	out, err := Build{KeyboardRepo: keyboards, SwitchRepo: switches, KeycapSetRepo: sets}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(340.0+30+45, *out.TotalCost, 0.001, "only the selected plate and PCB count")
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *BuildToMCPSummarySuite) TestPriceHidden_SkipsComponentLookupsEntirely() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Switches:   []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 10}},
		KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	out, err := Build{KeyboardRepo: keyboards}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: false})
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *BuildToMCPSummarySuite) TestSwitchWithoutQuantity_ExcludedFromTotalRatherThanGuessed() {
	switchPrice := 70.0

	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Switches: []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 10}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	switches := mocks.NewMockSwitchRepository(s.T())
	switches.EXPECT().
		Get(mock.Anything, "alice", "sw-1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw-1",
			Purchases: repository.BulkPurchases{{Price: &switchPrice}},
		}, nil)

	out, err := Build{KeyboardRepo: keyboards, SwitchRepo: switches}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
}

func (s *BuildToMCPSummarySuite) TestSwitchNotFound_ReturnsError() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Switches: []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 10}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	switches := mocks.NewMockSwitchRepository(s.T())
	switches.EXPECT().
		Get(mock.Anything, "alice", "sw-1").
		Return(nil, repository.ErrNotFound)

	_, err := Build{KeyboardRepo: keyboards, SwitchRepo: switches}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToMCPSummarySuite) TestKitNotFound_ReturnsError() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		KeycapKits: []repository.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	sets := mocks.NewMockKeycapSetRepository(s.T())
	sets.EXPECT().
		Get(mock.Anything, "alice", "ks-1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks-1"}, nil)

	_, err := Build{KeyboardRepo: keyboards, KeycapSetRepo: sets}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToMCPSummarySuite) TestDuplicateKit_PricedOnceAndRoundedToCents() {
	kitPrice := 100.005

	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks-1", Kit: "kit-1"},
			{KeycapSet: "ks-1", Kit: "kit-1"},
		},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	sets := mocks.NewMockKeycapSetRepository(s.T())
	sets.EXPECT().
		Get(mock.Anything, "alice", "ks-1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks-1",
			Kits: map[string]repository.KeycapKit{"kit-1": {KitID: "kit-1", Purchase: repository.KeycapKitPurchase{Price: &kitPrice}}},
		}, nil).Once()

	out, err := Build{KeyboardRepo: keyboards, KeycapSetRepo: sets}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(100.01, *out.TotalCost, 0.0001)
}

func (s *BuildToMCPSummarySuite) TestSwitchRepositoryError_ReturnsError() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Switches: []repository.BuildSwitchEntry{{Switch: "sw-1", Count: 10}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	switches := mocks.NewMockSwitchRepository(s.T())
	switches.EXPECT().
		Get(mock.Anything, "alice", "sw-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := Build{KeyboardRepo: keyboards, SwitchRepo: switches}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().Error(err)
}

func (s *BuildToMCPSuite) TestNonOwnerShowPriceToOthersFalse_PriceOnlyStabs_OmitsStabs() {
	out := Build{}.ToMCP(repository.Build{
		ID:         "build-1",
		Keyboard:   "kb-1",
		Visibility: repository.VisibilityPublic,
		Stabs:      &repository.BuildStabs{Price: new(12.5)},
	}, false, repository.ProfilePreferences{})

	s.Nil(out.Stabs)
}

func (s *BuildToMCPSuite) TestECBuild_MapsTypeAndPartsBothWays() {
	in := schema.BuildInput{Keyboard: "kb-1", BuildType: "EC", ECParts: []schema.BuildECPartEntry{{ECPart: "p-1", Count: 60}}, Visibility: "private"}

	b := Build{}.FromMCP(in)
	s.Equal("EC", b.BuildType)
	s.Equal([]repository.BuildECPartEntry{{ECPart: "p-1", Count: 60}}, b.ECParts)

	out := Build{}.ToMCP(b, true, repository.ProfilePreferences{})
	s.Equal("EC", out.BuildType)
	s.Equal([]schema.BuildECPartEntry{{ECPart: "p-1", Count: 60}}, out.ECParts)
}

func (s *BuildToMCPSuite) TestStoredWithoutBuildType_ReadsAsMX() {
	s.Equal("MX", Build{}.ToMCP(repository.Build{ID: "b-1"}, true, repository.ProfilePreferences{}).BuildType)
}

func (s *BuildToMCPSummarySuite) TestPriceShown_IncludesECParts() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1", BuildType: "EC",
		ECParts: []repository.BuildECPartEntry{{ECPart: "p-1", Count: 60}, {ECPart: "p-2", Count: 60}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1", Purchase: repository.KeyboardPurchase{Price: floatPtr(300)}}, nil)
	parts := mocks.NewMockECPartRepository(s.T())
	parts.EXPECT().Get(mock.Anything, "alice", "p-1").
		Return(&repository.ECPart{ID: "p-1", Purchases: repository.BulkPurchases{{Price: floatPtr(36), Quantity: new(72)}}}, nil)
	parts.EXPECT().Get(mock.Anything, "alice", "p-2").
		Return(&repository.ECPart{ID: "p-2", Purchases: repository.BulkPurchases{{Price: floatPtr(20)}}}, nil)

	out, err := Build{KeyboardRepo: keyboards, ECPartRepo: parts}.ToMCPSummary(
		context.Background(), b, true, repository.ProfilePreferences{ShowPriceToMe: true})
	s.Require().NoError(err)

	s.Equal("EC", out.BuildType)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(300+30, *out.TotalCost, 0.001, "a part without a quantity is left out")
}
