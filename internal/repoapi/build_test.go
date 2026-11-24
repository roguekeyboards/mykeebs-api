package repoapi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

func fullRepoBuild() repository.Build {
	return repository.Build{
		UserID:   "alice",
		ID:       "build1",
		Keyboard: "kb1",
		Plate:    strPtr("p1"),
		PCB:      strPtr("b1"),
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
			{Switch: "sw1", Count: 70},
		},
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks1", Kit: "kit1"},
		},
		BuildDate:  strPtr("2026-01-15"),
		Notes:      strPtr("first build"),
		Visibility: repository.VisibilityPrivate,
	}
}

type BuildToAPISuite struct {
	suite.Suite
}

func TestBuildToAPISuite(t *testing.T) {
	suite.Run(t, new(BuildToAPISuite))
}

type buildToAPIDeps struct {
	images         *mocks.MockBuildImageStore
	kitImages      *mocks.MockKeycapKitImageStore
	keyboardImages *mocks.MockKeyboardImageStore
	switchImages   *mocks.MockSwitchImageStore
	ecPartImages   *mocks.MockECPartImageStore
	keyboardRepo   *mocks.MockKeyboardRepository
	switchRepo     *mocks.MockSwitchRepository
	ecPartRepo     *mocks.MockECPartRepository
	keycapSetRepo  *mocks.MockKeycapSetRepository
}

func newBuildToAPIDeps(t interface {
	mock.TestingT
	Cleanup(func())
}) buildToAPIDeps {
	d := buildToAPIDeps{
		images:         mocks.NewMockBuildImageStore(t),
		kitImages:      mocks.NewMockKeycapKitImageStore(t),
		keyboardImages: mocks.NewMockKeyboardImageStore(t),
		switchImages:   mocks.NewMockSwitchImageStore(t),
		ecPartImages:   mocks.NewMockECPartImageStore(t),
		keyboardRepo:   mocks.NewMockKeyboardRepository(t),
		switchRepo:     mocks.NewMockSwitchRepository(t),
		ecPartRepo:     mocks.NewMockECPartRepository(t),
		keycapSetRepo:  mocks.NewMockKeycapSetRepository(t),
	}
	return d
}

func (d buildToAPIDeps) call(ctx context.Context, b repository.Build) (api.Build, error) {
	return d.callWithPrefs(ctx, b, true, repository.ProfilePreferences{})
}

func (d buildToAPIDeps) mapper() Build {
	return Build{
		Images:         d.images,
		KitImages:      d.kitImages,
		KeyboardImages: d.keyboardImages,
		SwitchImages:   d.switchImages,
		ECPartImages:   d.ecPartImages,
		KeyboardRepo:   d.keyboardRepo,
		SwitchRepo:     d.switchRepo,
		ECPartRepo:     d.ecPartRepo,
		KeycapSetRepo:  d.keycapSetRepo,
	}
}

func (d buildToAPIDeps) callWithPrefs(ctx context.Context, b repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Build, error) {
	return d.mapper().ToAPI(ctx, b, isOwner, ownerPrefs)
}

func (d buildToAPIDeps) expectFullyResolvable() {
	d.keyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1", Size: strPtr("75%"), Layout: strPtr("ANSI"),
			Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{{ID: "p1", Material: "Brass", Color: strPtr("Raw"), Thickness: floatPtr(1.5)}}),
			PCBs:   repository.KeyboardPCBsMap([]repository.KeyboardPCB{{ID: "b1", Firmware: strPtr("QMK/VIA"), Assembly: strPtr("Hot-swap")}}),
		}, nil)
	d.switchRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Profile: strPtr("Cherry"),
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}},
		}, nil)
}

func (s *BuildToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	b := fullRepoBuild()
	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Equal(b.ID, out.Id)
	s.Equal("kb1", out.Keyboard.Id)
	s.Equal("Keychron", out.Keyboard.Brand)
	s.Equal("Q1", out.Keyboard.Name)
	s.Equal(&api.BuildPlateRef{Id: "p1", Material: "Brass", Color: strPtr("Raw"), Thickness: floatPtr(1.5)}, out.Plate)
	s.Equal(&api.BuildPCBRef{Id: "b1", Firmware: strPtr("QMK/VIA"), Assembly: strPtr("Hot-swap")}, out.Pcb)
	s.Require().NotNil(out.CaseMountType)
	s.Equal(b.CaseMountType.Type, out.CaseMountType.Type)
	s.Equal(b.CaseMountType.Durometer, out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal(b.Stabs.Name, out.Stabs.Name)
	s.Equal(b.Stabs.MountType, out.Stabs.MountType)
	s.Equal(b.Stabs.Price, out.Stabs.Price)
	s.Equal(b.Foam, out.Foam)
	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 1)
	s.Equal("sw1", (*out.Switches)[0].Switch.Id)
	s.Equal("Oil King", (*out.Switches)[0].Switch.Name)
	s.Equal(70, (*out.Switches)[0].Count)
	s.Require().NotNil(out.KeycapSets)
	s.Require().Len(*out.KeycapSets, 1)
	s.Equal("ks1", (*out.KeycapSets)[0].Id)
	s.Equal("GMK", (*out.KeycapSets)[0].Brand)
	s.Equal("Olivia", (*out.KeycapSets)[0].Name)
	s.Require().Len((*out.KeycapSets)[0].Kits, 1)
	s.Equal("kit1", (*out.KeycapSets)[0].Kits[0].KitId)
	s.Equal("Base", (*out.KeycapSets)[0].Kits[0].Name)
	s.Require().NotNil(out.BuildDate)
	s.Equal(*b.BuildDate, out.BuildDate.Format(dateLayout))
	s.Equal(b.Notes, out.Notes)
	s.Require().NotNil(out.Visibility)
	s.Equal(api.Visibility(b.Visibility), *out.Visibility)
}

func (s *BuildToAPISuite) TestNonOwner_OmitsVisibility() {
	b := repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPublic}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := d.callWithPrefs(context.Background(), b, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Nil(out.Visibility)
}

func (s *BuildToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	b := repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.Plate)
	s.Nil(out.Pcb)
	s.Nil(out.CaseMountType)
	s.Nil(out.Stabs)
	s.Nil(out.Foam)
	s.Nil(out.Switches)
	s.Nil(out.KeycapSets)
	s.Nil(out.BuildDate)
	s.Nil(out.Notes)
	s.Nil(out.Images)
}

func (s *BuildToAPISuite) TestMalformedStoredBuildDate_ReturnsError() {
	b := fullRepoBuild()
	b.BuildDate = strPtr("not-a-date")

	d := newBuildToAPIDeps(s.T())
	_, err := d.call(context.Background(), b)

	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestImagesPopulated_SignsURLPerImage() {
	b := fullRepoBuild()
	img1 := repository.BuildImageKey("builds/alice/build1/images/img1")
	img2 := repository.BuildImageKey("builds/alice/build1/images/img2")
	b.Images = map[string]repository.BuildImageEntry{
		"img1": {Path: img1, Seq: 0},
		"img2": {Path: img2, Seq: 1},
	}

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()
	d.images.EXPECT().SignGetBuildImage(mock.Anything, img1).Return("https://example.com/img1", nil)
	d.images.EXPECT().SignGetBuildImage(mock.Anything, img2).Return("https://example.com/img2", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Require().Len(*out.Images, 2)
	s.Equal("img1", (*out.Images)[0].ImageId)
	s.Equal("https://example.com/img1", *(*out.Images)[0].Url)
	s.Equal("img2", (*out.Images)[1].ImageId)
	s.Equal("https://example.com/img2", *(*out.Images)[1].Url)
}

func (s *BuildToAPISuite) TestSignFails_ReturnsError() {
	b := fullRepoBuild()
	imgPath := repository.BuildImageKey("builds/alice/build1/images/img1")
	b.Images = map[string]repository.BuildImageEntry{"img1": {Path: imgPath}}

	d := newBuildToAPIDeps(s.T())
	d.images.EXPECT().SignGetBuildImage(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)

	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKeyboardNotFound_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(nil, repository.ErrNotFound)

	_, err := d.call(context.Background(), b)
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToAPISuite) TestKeyboardRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestSwitchNotFound_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(nil, repository.ErrNotFound)

	_, err := d.call(context.Background(), b)
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToAPISuite) TestMultipleSwitchEntries_ResolvesEachInOrder() {
	b := fullRepoBuild()
	b.Switches = []repository.BuildSwitchEntry{
		{Switch: "sw2", Count: 1},
		{Switch: "sw1", Count: 70},
	}

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw2").
		Return(&repository.Switch{UserID: "alice", ID: "sw2", Brand: "Cherry", Name: "MX Black", Type: "Linear"}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 2)
	s.Equal("sw2", (*out.Switches)[0].Switch.Id)
	s.Equal(1, (*out.Switches)[0].Count)
	s.Equal("sw1", (*out.Switches)[1].Switch.Id)
	s.Equal(70, (*out.Switches)[1].Count)
}

func (s *BuildToAPISuite) TestSwitchRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKeycapSetNotFound_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(nil, repository.ErrNotFound)

	_, err := d.call(context.Background(), b)
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToAPISuite) TestKeycapSetRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKitNotFoundInResolvedKeycapSet_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: map[string]repository.KeycapKit{"other-kit": {KitID: "other-kit", Name: "Base"}}}, nil)

	_, err := d.call(context.Background(), b)
	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildToAPISuite) TestKitsGroupedBySetInFirstAppearanceOrder_DuplicatesCollapsed() {
	b := fullRepoBuild()
	b.KeycapKits = []repository.BuildKeycapKitEntry{
		{KeycapSet: "ks2", Kit: "kitA"},
		{KeycapSet: "ks1", Kit: "kit1"},
		{KeycapSet: "ks2", Kit: "kitB"},
		{KeycapSet: "ks2", Kit: "kitA"},
	}

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks2").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks2", Brand: "ePBT", Name: "Kuro",
			Kits: map[string]repository.KeycapKit{
				"kitA": {KitID: "kitA", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: floatPtr(100)}},
				"kitB": {KitID: "kitB", Name: "Novelties", Purchase: repository.KeycapKitPurchase{Price: floatPtr(30)}},
			},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.KeycapSets)
	s.Require().Len(*out.KeycapSets, 2)
	s.Equal("ks2", (*out.KeycapSets)[0].Id)
	s.Require().Len((*out.KeycapSets)[0].Kits, 2)
	s.Equal("kitA", (*out.KeycapSets)[0].Kits[0].KitId)
	s.Equal("kitB", (*out.KeycapSets)[0].Kits[1].KitId)
	s.Equal("ks1", (*out.KeycapSets)[1].Id)
	s.Require().Len((*out.KeycapSets)[1].Kits, 1)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(142.5, *out.TotalCost, 0.0001, "kitA is priced once; stabs add 12.5")
}

func (s *BuildToAPISuite) TestKitWithImage_SignsURL() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	imgPath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base", ImagePath: &imgPath}},
		}, nil)
	d.kitImages.EXPECT().SignGet(mock.Anything, imgPath).Return("https://example.com/kit1.png", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.KeycapSets)
	s.Require().Len(*out.KeycapSets, 1)
	s.Require().Len((*out.KeycapSets)[0].Kits, 1)
	s.Require().NotNil((*out.KeycapSets)[0].Kits[0].ImageUrl)
	s.Equal("https://example.com/kit1.png", *(*out.KeycapSets)[0].Kits[0].ImageUrl)
}

func (s *BuildToAPISuite) TestKitImageSignFails_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	imgPath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image/up1")
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base", ImagePath: &imgPath}},
		}, nil)
	d.kitImages.EXPECT().SignGet(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKeyboardWithImage_SignsURLForFirstImage() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	imgPath := repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Images: repository.KeyboardImagesMap([]repository.KeyboardImage{
				{ImageID: "img1", Path: imgPath},
				{ImageID: "img2", Path: repository.KeyboardImageKey("keyboards/alice/kb1/images/img2")},
			}),
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}}}, nil)
	d.keyboardImages.EXPECT().SignGetKeyboardImage(mock.Anything, imgPath).Return("https://example.com/kb1-img1.png", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Keyboard)
	s.Require().NotNil(out.Keyboard.ImageUrl)
	s.Equal("https://example.com/kb1-img1.png", *out.Keyboard.ImageUrl)
}

func (s *BuildToAPISuite) TestKeyboardWithoutImages_OmitsImageUrl() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Keyboard)
	s.Nil(out.Keyboard.ImageUrl)
}

func (s *BuildToAPISuite) TestKeyboardImageSignFails_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	imgPath := repository.KeyboardImageKey("keyboards/alice/kb1/images/img1")
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Images: repository.KeyboardImagesMap([]repository.KeyboardImage{{ImageID: "img1", Path: imgPath}}),
		}, nil)
	d.keyboardImages.EXPECT().SignGetKeyboardImage(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestSwitchWithImage_SignsURL() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	imgPath := repository.SwitchImageKey("switches/alice/sw1/image/up1")
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear", ImagePath: &imgPath}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}}}, nil)
	d.switchImages.EXPECT().SignGet(mock.Anything, imgPath).Return("https://example.com/sw1.png", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 1)
	s.Require().NotNil((*out.Switches)[0].Switch)
	s.Require().NotNil((*out.Switches)[0].Switch.ImageUrl)
	s.Equal("https://example.com/sw1.png", *(*out.Switches)[0].Switch.ImageUrl)
}

func (s *BuildToAPISuite) TestSwitchWithoutImage_OmitsImageUrl() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 1)
	s.Require().NotNil((*out.Switches)[0].Switch)
	s.Nil((*out.Switches)[0].Switch.ImageUrl)
}

func (s *BuildToAPISuite) TestSwitchImageSignFails_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	imgPath := repository.SwitchImageKey("switches/alice/sw1/image/up1")
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear", ImagePath: &imgPath}, nil)
	d.switchImages.EXPECT().SignGet(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestTotalCost_SumsKeyboardSwitchesKitsAndStabs() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45), Quantity: intPtr(90)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+35+150+12.5, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestTotalCost_AddsOnlySelectedPlateAndPCB() {
	b := repository.Build{
		UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate,
		Plate: strPtr("p2"), PCB: strPtr("b1"),
	}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
			Plates: repository.KeyboardPlatesMap([]repository.KeyboardPlate{
				{ID: "p1", Material: "AL", Purchase: repository.KeyboardPurchase{Price: floatPtr(40)}},
				{ID: "p2", Material: "PC", Purchase: repository.KeyboardPurchase{Price: floatPtr(30)}},
			}),
			PCBs: repository.KeyboardPCBsMap([]repository.KeyboardPCB{
				{ID: "b1", Purchase: repository.KeyboardPurchase{Price: floatPtr(45)}},
				{ID: "b2", Purchase: repository.KeyboardPurchase{Price: floatPtr(60)}},
			}),
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+30+45, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestPartsNoLongerOnKeyboard_OmittedAndUnpriced() {
	b := repository.Build{
		UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate,
		Plate: strPtr("gone"), PCB: strPtr("gone"),
	}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
			Plates:   repository.KeyboardPlatesMap([]repository.KeyboardPlate{{ID: "p1", Material: "AL", Purchase: repository.KeyboardPurchase{Price: floatPtr(40)}}}),
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.Plate)
	s.Nil(out.Pcb)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(200, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestTotalCost_RoundedToCents() {
	b := fullRepoBuild()
	b.Stabs = nil

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(10), Quantity: intPtr(3)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(233.33, *out.TotalCost, 1e-9, "70 of 3 switches bought for 10")
}

func (s *BuildToAPISuite) TestTotalCost_SwitchesCostedAtAverageAcrossPurchases() {
	b := fullRepoBuild()
	b.Stabs = nil

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{
				{Price: floatPtr(36), Quantity: intPtr(90)},
				{Price: floatPtr(54), Quantity: intPtr(90)},
			},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(35.0, *out.TotalCost, 1e-9)
}

func (s *BuildToAPISuite) TestTotalCost_SwitchPriceWithoutQuantity_ExcludedFromSum() {
	b := fullRepoBuild()
	b.Stabs = nil

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
}

func (s *BuildToAPISuite) TestTotalCost_UnknownComponentsExcludedNotZeroed() {
	b := fullRepoBuild()
	b.Stabs = nil

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45), Quantity: intPtr(90)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {KitID: "kit1", Name: "Base"}},
		}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(35, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestTotalCost_NoPricedComponents_OmitsField() {
	b := repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
}

func (s *BuildToAPISuite) TestNonOwnerShowPriceToOthersFalse_OmitsStabsPriceAndTotalCost() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45), Quantity: intPtr(90)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := d.callWithPrefs(context.Background(), b, false, repository.ProfilePreferences{ShowPriceToOthers: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Stabs)
	s.Nil(out.Stabs.Price)
	s.Equal(b.Stabs.Name, out.Stabs.Name)
	s.Equal(b.Stabs.MountType, out.Stabs.MountType)
	s.Nil(out.TotalCost)
	s.Nil(out.Stabs.Currency)
	s.Nil(out.Currency)
}

func (s *BuildToAPISuite) TestNonOwnerShowPriceToOthersTrue_IncludesStabsPriceAndTotalCost() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45), Quantity: intPtr(90)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := d.callWithPrefs(context.Background(), b, false, repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true})
	s.Require().NoError(err)

	s.Require().NotNil(out.Stabs)
	s.Equal(b.Stabs.Price, out.Stabs.Price)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+35+150+12.5, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Stabs.Currency)
	s.Equal("EUR", *out.Stabs.Currency)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func (s *BuildToAPISuite) TestOwner_AlwaysIncludesStabsPriceAndTotalCostRegardlessOfShowPriceToMe() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchases: repository.BulkPurchases{{Price: floatPtr(45), Quantity: intPtr(90)}},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: map[string]repository.KeycapKit{"kit1": {
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := d.callWithPrefs(context.Background(), b, true, repository.ProfilePreferences{Currency: "EUR", ShowPriceToMe: false})
	s.Require().NoError(err)

	s.Require().NotNil(out.Stabs)
	s.Equal(b.Stabs.Price, out.Stabs.Price)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+35+150+12.5, *out.TotalCost, 0.0001)
	s.Require().NotNil(out.Stabs.Currency)
	s.Equal("EUR", *out.Stabs.Currency)
	s.Require().NotNil(out.Currency)
	s.Equal("EUR", *out.Currency)
}

func fullAPIBuildInput() api.BuildInput {
	return api.BuildInput{
		Keyboard: "kb1",
		Plate:    strPtr("p1"),
		Pcb:      strPtr("b1"),
		CaseMountType: &api.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &api.BuildStabsInput{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: &[]api.BuildSwitchEntry{
			{Switch: "sw1", Count: 70},
		},
		KeycapKits: &[]api.BuildKeycapKitEntry{
			{KeycapSet: "ks1", Kit: "kit1"},
		},
		Notes:      strPtr("first build"),
		Visibility: api.Visibility(repository.VisibilityPrivate),
	}
}

type BuildToRepoSuite struct {
	suite.Suite
}

func TestBuildToRepoSuite(t *testing.T) {
	suite.Run(t, new(BuildToRepoSuite))
}

func (s *BuildToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIBuildInput()
	out := Build{}.ToRepo(in)

	s.Equal(in.Keyboard, out.Keyboard)
	s.Equal(in.Plate, out.Plate)
	s.Equal(in.Pcb, out.PCB)
	s.Require().NotNil(out.CaseMountType)
	s.Equal(in.CaseMountType.Type, out.CaseMountType.Type)
	s.Equal(in.CaseMountType.Durometer, out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal(in.Stabs.Name, out.Stabs.Name)
	s.Equal(in.Stabs.MountType, out.Stabs.MountType)
	s.Equal(in.Stabs.Price, out.Stabs.Price)
	s.Equal(in.Foam, out.Foam)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw1", out.Switches[0].Switch)
	s.Equal(70, out.Switches[0].Count)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks1", out.KeycapKits[0].KeycapSet)
	s.Equal("kit1", out.KeycapKits[0].Kit)
	s.Equal(in.Notes, out.Notes)
	s.Equal(repository.Visibility(in.Visibility), out.Visibility)
	s.Empty(out.UserID)
	s.Empty(out.ID)
	s.Nil(out.Images, "a build write never carries images")
}

func (s *BuildToRepoSuite) TestAllOptionalFieldsNil_MapsToNil() {
	in := api.BuildInput{Keyboard: "kb1", Visibility: api.Visibility(repository.VisibilityPrivate)}

	out := Build{}.ToRepo(in)

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

func (s *BuildToAPISuite) TestStripPrices_ClearsStabsPriceAndTotalCostKeepsRest() {
	price, total, currency, name := 12.5, 200.0, "EUR", "Durock V2"
	out := api.Build{
		Id:        "build1",
		TotalCost: &total,
		Currency:  &currency,
		Stabs:     &api.BuildStabs{Name: &name, Price: &price, Currency: &currency},
	}

	Build{}.StripPrices(&out)

	s.Nil(out.TotalCost)
	s.Nil(out.Currency)
	s.Require().NotNil(out.Stabs)
	s.Nil(out.Stabs.Price)
	s.Nil(out.Stabs.Currency)
	s.Equal(&name, out.Stabs.Name)
}

func (s *BuildToAPISuite) TestStripPrices_StabsOnlyPriced_DropsStabs() {
	price, currency := 12.5, "EUR"
	out := api.Build{Id: "build1", Stabs: &api.BuildStabs{Price: &price, Currency: &currency}}

	Build{}.StripPrices(&out)

	s.Nil(out.Stabs)
}

func (s *BuildToAPISuite) TestStoredWithoutBuildType_ReadsAsMX() {
	b := fullRepoBuild()
	b.BuildType = ""

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)
	s.Equal("MX", out.BuildType)
}

func (s *BuildToAPISuite) TestECBuild_ResolvesPartsSignsImagesAndPricesThem() {
	b := repository.Build{
		UserID: "alice", ID: "b1", Keyboard: "kb1", BuildType: "EC",
		ECParts: []repository.BuildECPartEntry{{ECPart: "domes", Count: 60}, {ECPart: "sliders", Count: 60}},
	}
	domesImage := repository.ECPartImageKey("ec-parts/alice/domes/image/u1")

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Purchase: repository.KeyboardPurchase{Price: floatPtr(300)}}, nil)
	d.ecPartRepo.EXPECT().Get(mock.Anything, "alice", "domes").
		Return(&repository.ECPart{
			ID: "domes", Brand: "DynaCap", Name: "Heavy", PartType: repository.ECPartTypeDomes,
			ImagePath: &domesImage, ImageStatus: repository.ImageStatusApproved,
			Purchases: repository.BulkPurchases{{Price: floatPtr(36), Quantity: intPtr(72)}},
		}, nil)
	d.ecPartRepo.EXPECT().Get(mock.Anything, "alice", "sliders").
		Return(&repository.ECPart{ID: "sliders", Brand: "DynaCap", Name: "Sliders", PartType: repository.ECPartTypeSliders}, nil)
	d.ecPartImages.EXPECT().SignGet(mock.Anything, domesImage).Return("https://example.com/domes", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Equal("EC", out.BuildType)
	s.Nil(out.Switches)
	s.Equal(&[]api.BuildECPartEntryResolved{
		{Count: 60, EcPart: api.BuildECPartRef{Id: "domes", Brand: "DynaCap", Name: "Heavy", PartType: api.ECPartTypeDomes, ImageUrl: strPtr("https://example.com/domes")}},
		{Count: 60, EcPart: api.BuildECPartRef{Id: "sliders", Brand: "DynaCap", Name: "Sliders", PartType: api.ECPartTypeSliders}},
	}, out.EcParts)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(300+30, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestECPartNotFound_ReturnsError() {
	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(&repository.Keyboard{UserID: "alice", ID: "kb1"}, nil)
	d.ecPartRepo.EXPECT().Get(mock.Anything, "alice", "p1").Return(nil, repository.ErrNotFound)

	_, err := d.call(context.Background(), repository.Build{UserID: "alice", ID: "b1", Keyboard: "kb1", BuildType: "EC", ECParts: []repository.BuildECPartEntry{{ECPart: "p1", Count: 1}}})

	s.Error(err)
}
