package moderation

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/imagekey"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	repomocks "github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type ImagesSuite struct {
	suite.Suite
	keyboards  *repomocks.MockKeyboardRepository
	builds     *repomocks.MockBuildRepository
	switches   *repomocks.MockSwitchRepository
	ecParts    *repomocks.MockECPartRepository
	keycapSets *repomocks.MockKeycapSetRepository
	profiles   *repomocks.MockProfileRepository
	images     *Images
}

func TestImagesSuite(t *testing.T) {
	suite.Run(t, new(ImagesSuite))
}

func (s *ImagesSuite) SetupTest() {
	s.keyboards = repomocks.NewMockKeyboardRepository(s.T())
	s.builds = repomocks.NewMockBuildRepository(s.T())
	s.switches = repomocks.NewMockSwitchRepository(s.T())
	s.ecParts = repomocks.NewMockECPartRepository(s.T())
	s.keycapSets = repomocks.NewMockKeycapSetRepository(s.T())
	s.profiles = repomocks.NewMockProfileRepository(s.T())
	s.images = &Images{Keyboards: s.keyboards, Builds: s.builds, Switches: s.switches, ECParts: s.ecParts, KeycapSets: s.keycapSets, Profiles: s.profiles}
}

func (s *ImagesSuite) status(key string) (repository.ImageStatus, bool) {
	ref, err := imagekey.Parse(key)
	s.Require().NoError(err)
	status, current, err := s.images.Status(s.T().Context(), ref, key)
	s.Require().NoError(err)
	return status, current
}

func (s *ImagesSuite) setStatus(key string) bool {
	ref, err := imagekey.Parse(key)
	s.Require().NoError(err)
	reason := "Explicit"
	ok, err := s.images.SetStatus(s.T().Context(), ref, key, repository.ImageStatusPending, repository.ImageStatusRejected, &reason)
	s.Require().NoError(err)
	return ok
}

func (s *ImagesSuite) TestKeyboard() {
	key := "keyboards/alice/kb1/images/img1"
	s.keyboards.EXPECT().Get(mock.Anything, "alice", "kb1").Return(&repository.Keyboard{Images: map[string]repository.KeyboardImageEntry{
		"img1": {Path: repository.KeyboardImageKey(key), Status: repository.ImageStatusPending},
	}}, nil)
	s.keyboards.EXPECT().SetImageStatus(mock.Anything, "alice", "kb1", "img1", repository.KeyboardImageKey(key), repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(true, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusPending, status)
	s.True(s.setStatus(key))
}

func (s *ImagesSuite) TestKeyboard_ImageGone_NotCurrent() {
	s.keyboards.EXPECT().Get(mock.Anything, "alice", "kb1").Return(&repository.Keyboard{}, nil)

	_, current := s.status("keyboards/alice/kb1/images/img1")
	s.False(current)
}

func (s *ImagesSuite) TestBuild() {
	key := "builds/alice/b1/images/img1"
	s.builds.EXPECT().Get(mock.Anything, "alice", "b1").Return(&repository.Build{Images: map[string]repository.BuildImageEntry{
		"img1": {Path: repository.BuildImageKey(key)},
	}}, nil)
	s.builds.EXPECT().SetImageStatus(mock.Anything, "alice", "b1", "img1", repository.BuildImageKey(key), repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(true, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusApproved, status, "no stored status reads as approved")
	s.True(s.setStatus(key))
}

func (s *ImagesSuite) TestSwitch() {
	key := "switches/alice/sw1/image/up1"
	path := repository.SwitchImageKey(key)
	s.switches.EXPECT().Get(mock.Anything, "alice", "sw1").Return(&repository.Switch{ImagePath: &path, ImageStatus: repository.ImageStatusPending}, nil)
	s.switches.EXPECT().SetImageStatus(mock.Anything, "alice", "sw1", path, repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(true, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusPending, status)
	s.True(s.setStatus(key))
}

func (s *ImagesSuite) TestSwitch_Replaced_NotCurrent() {
	path := repository.SwitchImageKey("switches/alice/sw1/image/up2")
	s.switches.EXPECT().Get(mock.Anything, "alice", "sw1").Return(&repository.Switch{ImagePath: &path}, nil)

	_, current := s.status("switches/alice/sw1/image/up1")
	s.False(current)
}

func (s *ImagesSuite) TestECPart() {
	key := "ec-parts/alice/p1/image/up1"
	path := repository.ECPartImageKey(key)
	s.ecParts.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ImagePath: &path, ImageStatus: repository.ImageStatusPending}, nil)
	s.ecParts.EXPECT().SetImageStatus(mock.Anything, "alice", "p1", path, repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(true, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusPending, status)
	s.True(s.setStatus(key))
}

func (s *ImagesSuite) TestECPart_Replaced_NotCurrent() {
	path := repository.ECPartImageKey("ec-parts/alice/p1/image/up2")
	s.ecParts.EXPECT().Get(mock.Anything, "alice", "p1").Return(&repository.ECPart{ImagePath: &path}, nil)

	_, current := s.status("ec-parts/alice/p1/image/up1")
	s.False(current)
}

func (s *ImagesSuite) TestKeycapKit() {
	key := "keycap-sets/alice/set1/kits/kit1/image/up1"
	path := repository.KeycapKitImageKey(key)
	s.keycapSets.EXPECT().Get(mock.Anything, "alice", "set1").Return(&repository.KeycapSet{Kits: map[string]repository.KeycapKit{
		"kit1": {ImagePath: &path, ImageStatus: repository.ImageStatusRejected},
	}}, nil)
	s.keycapSets.EXPECT().SetKitImageStatus(mock.Anything, "alice", "set1", "kit1", path, repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(false, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusRejected, status)
	s.False(s.setStatus(key))
}

func (s *ImagesSuite) TestProfile() {
	key := "profiles/alice/avatar/up1"
	path := repository.ProfileImageKey(key)
	s.profiles.EXPECT().Get(mock.Anything, "alice").Return(&repository.Profile{AvatarPath: &path, AvatarStatus: repository.ImageStatusPending}, nil)
	s.profiles.EXPECT().SetAvatarStatus(mock.Anything, "alice", path, repository.ImageStatusPending, repository.ImageStatusRejected, mock.Anything).Return(true, nil)

	status, current := s.status(key)
	s.True(current)
	s.Equal(repository.ImageStatusPending, status)
	s.True(s.setStatus(key))
}

func (s *ImagesSuite) TestItemNotFound_NotCurrent() {
	s.profiles.EXPECT().Get(mock.Anything, "alice").Return(nil, repository.ErrNotFound)

	_, current := s.status("profiles/alice/avatar/up1")
	s.False(current)
}

func (s *ImagesSuite) TestReadError_Returned() {
	key := "profiles/alice/avatar/up1"
	s.profiles.EXPECT().Get(mock.Anything, "alice").Return(nil, errors.New("dynamodb down"))
	ref, err := imagekey.Parse(key)
	s.Require().NoError(err)

	_, _, err = s.images.Status(s.T().Context(), ref, key)
	s.Error(err)
}
