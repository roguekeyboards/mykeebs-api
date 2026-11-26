package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/availability"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ImageModerationSuite covers who sees which images by moderation status:
// the owner sees every image with its status; anyone else sees approved
// images only, with no status.
type ImageModerationSuite struct {
	suite.Suite
}

func TestImageModerationSuite(t *testing.T) {
	suite.Run(t, new(ImageModerationSuite))
}

func ptr[T any](v T) *T { return &v }

func (s *ImageModerationSuite) keyboard(statuses ...repository.ImageStatus) repository.Keyboard {
	images := map[string]repository.KeyboardImageEntry{}
	for i, st := range statuses {
		id := string(st)
		if id == "" {
			id = "legacy"
		}
		images[id] = repository.KeyboardImageEntry{Path: repository.KeyboardImageKey("k/" + id), Seq: i, Status: st}
	}
	return repository.Keyboard{UserID: "alice", ID: "kb1", Images: images}
}

func (s *ImageModerationSuite) TestKeyboardHasImages() {
	pendingOnly := s.keyboard(repository.ImageStatusPending)
	s.True(Keyboard{}.ToMCP(pendingOnly, true, repository.ProfilePreferences{}).HasImages)
	s.False(Keyboard{}.ToMCP(pendingOnly, false, repository.ProfilePreferences{}).HasImages)
	s.False(Keyboard{}.ToMCPSummary(pendingOnly, false, repository.ProfilePreferences{}).HasImages)

	legacy := s.keyboard("")
	s.True(Keyboard{}.ToMCPSummary(legacy, false, repository.ProfilePreferences{}).HasImages, "no status reads as approved")
}

func (s *ImageModerationSuite) TestKeyboardImages_Owner_EveryImageWithStatus() {
	kb := s.keyboard(repository.ImageStatusPending, repository.ImageStatusApproved, repository.ImageStatusRejected)
	e := kb.Images["rejected"]
	e.RejectionReason = ptr("Explicit")
	kb.Images["rejected"] = e

	s.Equal([]schema.KeyboardImage{
		{ImageID: "pending", Status: ptr("pending")},
		{ImageID: "approved", Status: ptr("approved")},
		{ImageID: "rejected", Status: ptr("rejected"), RejectionReason: ptr("Explicit")},
	}, Keyboard{}.ImagesToMCP(kb, true))
}

func (s *ImageModerationSuite) TestKeyboardImages_NonOwner_ApprovedOnly() {
	kb := s.keyboard(repository.ImageStatusPending, repository.ImageStatusApproved, repository.ImageStatusRejected)

	s.Equal([]schema.KeyboardImage{{ImageID: "approved"}}, Keyboard{}.ImagesToMCP(kb, false))
}

func (s *ImageModerationSuite) TestKeyboardImages_NonOwner_NoneApproved_EmptyNotNil() {
	s.Equal([]schema.KeyboardImage{}, Keyboard{}.ImagesToMCP(s.keyboard(repository.ImageStatusPending), false))
}

func (s *ImageModerationSuite) build(statuses ...repository.ImageStatus) repository.Build {
	images := map[string]repository.BuildImageEntry{}
	for i, st := range statuses {
		images[string(st)] = repository.BuildImageEntry{Path: repository.BuildImageKey("b/" + string(st)), Seq: i, Status: st}
	}
	return repository.Build{UserID: "alice", ID: "b1", Images: images}
}

func (s *ImageModerationSuite) TestBuildHasImages() {
	rejectedOnly := s.build(repository.ImageStatusRejected)
	s.True(Build{}.ToMCP(rejectedOnly, true, repository.ProfilePreferences{}).HasImages)
	s.False(Build{}.ToMCP(rejectedOnly, false, repository.ProfilePreferences{}).HasImages)
}

func (s *ImageModerationSuite) TestBuildImages() {
	b := s.build(repository.ImageStatusPending, repository.ImageStatusApproved)

	s.Equal([]schema.BuildImage{
		{ImageID: "pending", Status: ptr("pending")},
		{ImageID: "approved", Status: ptr("approved")},
	}, Build{}.ImagesToMCP(b, true))
	s.Equal([]schema.BuildImage{{ImageID: "approved"}}, Build{}.ImagesToMCP(b, false))
}

func (s *ImageModerationSuite) TestSwitch() {
	path := repository.SwitchImageKey("s/1")
	sw := repository.Switch{ImagePath: &path, ImageStatus: repository.ImageStatusRejected, ImageRejectionReason: ptr("Violence")}

	owner := Switch{}.ToMCP(sw, true, repository.ProfilePreferences{}, availability.Usage{})
	s.True(owner.HasImage)
	s.Equal(ptr("rejected"), owner.ImageStatus)
	s.Equal(ptr("Violence"), owner.ImageRejectionReason)

	other := Switch{}.ToMCP(sw, false, repository.ProfilePreferences{}, availability.Usage{})
	s.False(other.HasImage)
	s.Nil(other.ImageStatus)
	s.Nil(other.ImageRejectionReason)
	s.False(Switch{}.ToMCPSummary(sw, false, repository.ProfilePreferences{}, availability.Usage{}).HasImage)
}

func (s *ImageModerationSuite) TestSwitch_NoImage_NoStatus() {
	s.Nil(Switch{}.ToMCP(repository.Switch{}, true, repository.ProfilePreferences{}, availability.Usage{}).ImageStatus)
}

func (s *ImageModerationSuite) TestKeycapKit() {
	path := repository.KeycapKitImageKey("k/1")
	kit := repository.KeycapKit{KitID: "kit1", ImagePath: &path, ImageStatus: repository.ImageStatusPending}
	set := repository.KeycapSet{Kits: map[string]repository.KeycapKit{"kit1": kit}, PrimaryKitID: ptr("kit1")}

	owner := KeycapSet{}.KitToMCP(kit, true, repository.ProfilePreferences{})
	s.True(owner.HasImage)
	s.Equal(ptr("pending"), owner.ImageStatus)
	s.True(KeycapSet{}.ToMCPSummary(set, true, repository.ProfilePreferences{}).PrimaryKitHasImage)

	other := KeycapSet{}.KitToMCP(kit, false, repository.ProfilePreferences{})
	s.False(other.HasImage)
	s.Nil(other.ImageStatus)
	s.False(KeycapSet{}.ToMCPSummary(set, false, repository.ProfilePreferences{}).PrimaryKitHasImage)
}

func (s *ImageModerationSuite) TestProfile() {
	path := repository.ProfileImageKey("p/1")
	prof := repository.Profile{AvatarPath: &path, AvatarStatus: repository.ImageStatusApproved}

	owner := Profile{}.ToMCP(prof, true)
	s.True(owner.HasAvatar)
	s.Equal(ptr("approved"), owner.AvatarStatus)

	other := Profile{}.ToMCP(prof, false)
	s.True(other.HasAvatar)
	s.Nil(other.AvatarStatus)

	prof.AvatarStatus = repository.ImageStatusPending
	s.False(Profile{}.ToMCPSummary(prof, false).HasAvatar)
	s.True(Profile{}.ToMCPSummary(prof, true).HasAvatar)
}
