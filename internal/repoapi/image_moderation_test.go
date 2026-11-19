package repoapi

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type ImageModerationSuite struct {
	suite.Suite
}

func TestImageModerationSuite(t *testing.T) {
	suite.Run(t, new(ImageModerationSuite))
}

const (
	legacyKey   = "k/legacy"
	pendingKey  = "k/pending"
	approvedKey = "k/approved"
	rejectedKey = "k/rejected"
)

func statusPtr(s api.ImageStatus) *api.ImageStatus { return &s }

func (s *ImageModerationSuite) keyboardImages() map[string]repository.KeyboardImageEntry {
	return map[string]repository.KeyboardImageEntry{
		"legacy":   {Path: legacyKey, Seq: 0},
		"pending":  {Path: pendingKey, Seq: 1, Status: repository.ImageStatusPending},
		"approved": {Path: approvedKey, Seq: 2, Status: repository.ImageStatusApproved},
		"rejected": {Path: rejectedKey, Seq: 3, Status: repository.ImageStatusRejected, RejectionReason: strPtr("Explicit")},
	}
}

func (s *ImageModerationSuite) keyboardMapper(signed ...repository.KeyboardImageKey) Keyboard {
	images := mocks.NewMockKeyboardImageStore(s.T())
	for _, key := range signed {
		images.EXPECT().SignGetKeyboardImage(mock.Anything, key).Return("https://example.com/"+string(key), nil)
	}
	return Keyboard{Images: images}
}

func (s *ImageModerationSuite) TestKeyboard_Owner_SeesEveryImageWithStatus_NoURLWhenRejected() {
	kb := fullRepoKeyboard()
	kb.Images = s.keyboardImages()

	out, err := s.keyboardMapper(legacyKey, pendingKey, approvedKey).ToAPI(s.T().Context(), kb, true, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Equal([]api.KeyboardImage{
		{ImageId: "legacy", Url: strPtr("https://example.com/" + legacyKey), Status: statusPtr(api.ImageStatusApproved)},
		{ImageId: "pending", Url: strPtr("https://example.com/" + pendingKey), Status: statusPtr(api.ImageStatusPending)},
		{ImageId: "approved", Url: strPtr("https://example.com/" + approvedKey), Status: statusPtr(api.ImageStatusApproved)},
		{ImageId: "rejected", Status: statusPtr(api.ImageStatusRejected), RejectionReason: strPtr("Explicit")},
	}, *out.Images)
}

func (s *ImageModerationSuite) TestKeyboard_NonOwner_SeesApprovedOnly_WithoutStatus() {
	kb := fullRepoKeyboard()
	kb.Images = s.keyboardImages()

	out, err := s.keyboardMapper(legacyKey, approvedKey).ToAPI(s.T().Context(), kb, false, repository.ProfilePreferences{})
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Equal([]api.KeyboardImage{
		{ImageId: "legacy", Url: strPtr("https://example.com/" + legacyKey)},
		{ImageId: "approved", Url: strPtr("https://example.com/" + approvedKey)},
	}, *out.Images)
}

func (s *ImageModerationSuite) TestKeyboard_NonOwner_NothingApproved_NoImages() {
	kb := fullRepoKeyboard()
	kb.Images = map[string]repository.KeyboardImageEntry{
		"pending": {Path: pendingKey, Status: repository.ImageStatusPending},
	}

	out, err := s.keyboardMapper().ToAPI(s.T().Context(), kb, false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Nil(out.Images)
}

func (s *ImageModerationSuite) buildMapper(signed ...repository.BuildImageKey) Build {
	images := mocks.NewMockBuildImageStore(s.T())
	for _, key := range signed {
		images.EXPECT().SignGetBuildImage(mock.Anything, key).Return("https://example.com/"+string(key), nil)
	}
	return Build{Images: images}
}

func (s *ImageModerationSuite) buildImages() []repository.BuildImage {
	return repository.SortedBuildImages(map[string]repository.BuildImageEntry{
		"pending":  {Path: pendingKey, Seq: 0, Status: repository.ImageStatusPending},
		"approved": {Path: approvedKey, Seq: 1, Status: repository.ImageStatusApproved},
		"rejected": {Path: rejectedKey, Seq: 2, Status: repository.ImageStatusRejected, RejectionReason: strPtr("unprocessable")},
	})
}

func (s *ImageModerationSuite) TestBuild_Owner_SeesEveryImageWithStatus() {
	out, err := s.buildMapper(pendingKey, approvedKey).imagesToAPI(s.T().Context(), s.buildImages(), true)
	s.Require().NoError(err)

	s.Equal([]api.BuildImage{
		{ImageId: "pending", Url: strPtr("https://example.com/" + pendingKey), Status: statusPtr(api.ImageStatusPending)},
		{ImageId: "approved", Url: strPtr("https://example.com/" + approvedKey), Status: statusPtr(api.ImageStatusApproved)},
		{ImageId: "rejected", Status: statusPtr(api.ImageStatusRejected), RejectionReason: strPtr("unprocessable")},
	}, *out)
}

func (s *ImageModerationSuite) TestBuild_NonOwner_SeesApprovedOnly() {
	out, err := s.buildMapper(approvedKey).imagesToAPI(s.T().Context(), s.buildImages(), false)
	s.Require().NoError(err)

	s.Equal([]api.BuildImage{{ImageId: "approved", Url: strPtr("https://example.com/" + approvedKey)}}, *out)
}

func (s *ImageModerationSuite) buildWithKeyboard(signed ...repository.KeyboardImageKey) Build {
	kb := fullRepoKeyboard()
	kb.Images = map[string]repository.KeyboardImageEntry{
		"rejected": {Path: rejectedKey, Seq: 0, Status: repository.ImageStatusRejected},
		"pending":  {Path: pendingKey, Seq: 1, Status: repository.ImageStatusPending},
		"approved": {Path: approvedKey, Seq: 2, Status: repository.ImageStatusApproved},
	}
	repo := mocks.NewMockKeyboardRepository(s.T())
	repo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(&kb, nil)
	images := mocks.NewMockKeyboardImageStore(s.T())
	for _, key := range signed {
		images.EXPECT().SignGetKeyboardImage(mock.Anything, key).Return("https://example.com/"+string(key), nil)
	}

	return Build{KeyboardRepo: repo, KeyboardImages: images}
}

func (s *ImageModerationSuite) TestBuildKeyboardRef_Owner_FirstImageNotRejected() {
	ref, _, err := s.buildWithKeyboard(pendingKey).keyboardRefToAPI(s.T().Context(), "alice", "kb1", true)
	s.Require().NoError(err)
	s.Equal(strPtr("https://example.com/"+pendingKey), ref.ImageUrl)
}

func (s *ImageModerationSuite) TestBuildKeyboardRef_NonOwner_FirstApprovedImage() {
	ref, _, err := s.buildWithKeyboard(approvedKey).keyboardRefToAPI(s.T().Context(), "alice", "kb1", false)
	s.Require().NoError(err)
	s.Equal(strPtr("https://example.com/"+approvedKey), ref.ImageUrl)
}

func (s *ImageModerationSuite) TestServedInRef() {
	s.True(servedInRef("", false), "no status reads as approved")
	s.True(servedInRef(repository.ImageStatusPending, true))
	s.False(servedInRef(repository.ImageStatusPending, false))
	s.False(servedInRef(repository.ImageStatusRejected, true))
}

func (s *ImageModerationSuite) switchMapper(signed ...repository.SwitchImageKey) Switch {
	images := mocks.NewMockSwitchImageStore(s.T())
	for _, key := range signed {
		images.EXPECT().SignGet(mock.Anything, key).Return("https://example.com/"+string(key), nil)
	}
	return Switch{Images: images}
}

func (s *ImageModerationSuite) switchWith(key string, status repository.ImageStatus, reason *string) repository.Switch {
	path := repository.SwitchImageKey(key)
	return repository.Switch{UserID: "alice", ID: "sw1", ImagePath: &path, ImageStatus: status, ImageRejectionReason: reason}
}

func (s *ImageModerationSuite) TestSwitch_Owner_Pending_URLAndStatus() {
	out, err := s.switchMapper(pendingKey).ToAPI(s.T().Context(), s.switchWith(pendingKey, repository.ImageStatusPending, nil), true, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Equal(&api.SwitchImage{Url: strPtr("https://example.com/" + pendingKey), Status: statusPtr(api.ImageStatusPending)}, out.Image)
}

func (s *ImageModerationSuite) TestSwitch_Owner_Rejected_ReasonNoURL() {
	out, err := s.switchMapper().ToAPI(s.T().Context(), s.switchWith(rejectedKey, repository.ImageStatusRejected, strPtr("Violence")), true, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Equal(&api.SwitchImage{Status: statusPtr(api.ImageStatusRejected), RejectionReason: strPtr("Violence")}, out.Image)
}

func (s *ImageModerationSuite) TestSwitch_NonOwner_Pending_NoImage() {
	out, err := s.switchMapper().ToAPI(s.T().Context(), s.switchWith(pendingKey, repository.ImageStatusPending, nil), false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Nil(out.Image)
}

func (s *ImageModerationSuite) TestSwitch_NonOwner_Approved_URLOnly() {
	out, err := s.switchMapper(approvedKey).ToAPI(s.T().Context(), s.switchWith(approvedKey, repository.ImageStatusApproved, nil), false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Equal(&api.SwitchImage{Url: strPtr("https://example.com/" + approvedKey)}, out.Image)
}

func (s *ImageModerationSuite) kitWith(key string, status repository.ImageStatus, reason *string) repository.KeycapKit {
	path := repository.KeycapKitImageKey(key)
	return repository.KeycapKit{KitID: "kit1", Name: "Base", ImagePath: &path, ImageStatus: status, ImageRejectionReason: reason}
}

func (s *ImageModerationSuite) TestKit_Owner_Rejected_ReasonNoURL() {
	ks := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := ks.KitToAPI(s.T().Context(), s.kitWith(rejectedKey, repository.ImageStatusRejected, strPtr("Hate Symbols")), true, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Equal(&api.KeycapKitImage{Status: statusPtr(api.ImageStatusRejected), RejectionReason: strPtr("Hate Symbols")}, out.Image)
}

func (s *ImageModerationSuite) TestKit_NonOwner_Rejected_NoImage() {
	ks := KeycapSet{Images: mocks.NewMockKeycapKitImageStore(s.T())}
	out, err := ks.KitToAPI(s.T().Context(), s.kitWith(rejectedKey, repository.ImageStatusRejected, strPtr("Hate Symbols")), false, repository.ProfilePreferences{})
	s.Require().NoError(err)
	s.Nil(out.Image)
}

func (s *ImageModerationSuite) profileWith(key string, status repository.ImageStatus, reason *string) repository.Profile {
	path := repository.ProfileImageKey(key)
	return repository.Profile{OwnerID: "alice", Username: "alice", AvatarPath: &path, AvatarStatus: status, AvatarRejectionReason: reason}
}

func (s *ImageModerationSuite) TestProfile_Owner_Rejected_ReasonNoURL() {
	p := Profile{Images: mocks.NewMockProfileImageStore(s.T())}
	out, err := p.ToAPI(s.T().Context(), s.profileWith(rejectedKey, repository.ImageStatusRejected, strPtr("unprocessable")), true)
	s.Require().NoError(err)
	s.Equal(&api.ProfileImage{Status: statusPtr(api.ImageStatusRejected), RejectionReason: strPtr("unprocessable")}, out.Avatar)
}

func (s *ImageModerationSuite) TestProfile_NonOwner_Pending_NoAvatar() {
	p := Profile{Images: mocks.NewMockProfileImageStore(s.T())}
	out, err := p.ToAPI(s.T().Context(), s.profileWith(pendingKey, repository.ImageStatusPending, nil), false)
	s.Require().NoError(err)
	s.Nil(out.Avatar)
}
