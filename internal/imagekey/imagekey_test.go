package imagekey

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type ParseSuite struct {
	suite.Suite
	ctx context.Context
}

func TestParseSuite(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}

func (s *ParseSuite) SetupTest() {
	s.ctx = apictx.WithUserID(context.Background(), "owner-1")
}

func (s *ParseSuite) parse(key string, err error) Ref {
	s.Require().NoError(err)
	ref, err := Parse(key)
	s.Require().NoError(err)
	return ref
}

func (s *ParseSuite) TestKeyboard() {
	key, err := repository.NewKeyboardImageKey(s.ctx, "kb-1", "img-1")
	s.Equal(Ref{Entity: Keyboard, OwnerID: "owner-1", ItemID: "kb-1", ImageID: "img-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestBuild() {
	key, err := repository.NewBuildImageKey(s.ctx, "build-1", "img-1")
	s.Equal(Ref{Entity: Build, OwnerID: "owner-1", ItemID: "build-1", ImageID: "img-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestSwitch() {
	key, err := repository.NewSwitchImageKey(s.ctx, "sw-1", "up-1")
	s.Equal(Ref{Entity: Switch, OwnerID: "owner-1", ItemID: "sw-1", ImageID: "up-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestECPart() {
	key, err := repository.NewECPartImageKey(s.ctx, "part-1", "up-1")
	s.Equal(Ref{Entity: ECPart, OwnerID: "owner-1", ItemID: "part-1", ImageID: "up-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestKeycapKit() {
	key, err := repository.NewKeycapKitImageKey(s.ctx, "set-1", "kit-1", "up-1")
	s.Equal(Ref{Entity: KeycapKit, OwnerID: "owner-1", ItemID: "set-1", KitID: "kit-1", ImageID: "up-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestProfile() {
	key, err := repository.NewProfileImageKey(s.ctx, "up-1")
	s.Equal(Ref{Entity: Profile, OwnerID: "owner-1", ImageID: "up-1"}, s.parse(string(key), err))
}

func (s *ParseSuite) TestUnknown() {
	for _, key := range []string{
		"",
		"keyboards",
		"widgets/owner-1/w-1/images/img-1",
		"keyboards/owner-1/kb-1/image/img-1",
		"keyboards/owner-1/kb-1/images/img-1/extra",
		"keyboards//kb-1/images/img-1",
		"keyboards/owner-1/kb-1/images/",
		"/keyboards/owner-1/kb-1/images/img-1",
		"switches/owner-1/sw-1/image",
		"ec-parts/owner-1/part-1/images/up-1",
		"keycap-sets/owner-1/set-1/kits/kit-1/image",
		"profiles/owner-1/avatar",
	} {
		_, err := Parse(key)
		s.ErrorIs(err, ErrUnknownKey, key)
	}
}
