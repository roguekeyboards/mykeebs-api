package repository_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type OwnerImagePrefixesSuite struct {
	suite.Suite
}

func TestOwnerImagePrefixesSuite(t *testing.T) {
	suite.Run(t, new(OwnerImagePrefixesSuite))
}

// Every key format must sit under one of the owner's prefixes, or an
// account delete would leave that kind of image behind.
func (s *OwnerImagePrefixesSuite) TestCoversEveryImageKeyFormat() {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	build, err := repository.NewBuildImageKey(ctx, "b1", "i1")
	s.Require().NoError(err)
	keyboard, err := repository.NewKeyboardImageKey(ctx, "k1", "i1")
	s.Require().NoError(err)
	kit, err := repository.NewKeycapKitImageKey(ctx, "ks1", "kit1", "u1")
	s.Require().NoError(err)
	profile, err := repository.NewProfileImageKey(ctx, "u1")
	s.Require().NoError(err)
	sw, err := repository.NewSwitchImageKey(ctx, "s1", "u1")
	s.Require().NoError(err)
	ecPart, err := repository.NewECPartImageKey(ctx, "p1", "u1")
	s.Require().NoError(err)

	prefixes := repository.OwnerImagePrefixes("alice")
	for _, key := range []string{string(build), string(keyboard), string(kit), string(profile), string(sw), string(ecPart)} {
		covered := false
		for _, p := range prefixes {
			covered = covered || strings.HasPrefix(key, p)
		}
		s.True(covered, key)
	}
}

func (s *OwnerImagePrefixesSuite) TestDoesNotMatchLongerOwnerID() {
	for _, p := range repository.OwnerImagePrefixes("alice") {
		s.False(strings.HasPrefix(strings.Replace(p, "alice", "alice2", 1)+"x", p), p)
	}
}
