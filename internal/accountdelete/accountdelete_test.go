package accountdelete_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	admocks "github.com/roguekeyboards/mykeebs-api/internal/accountdelete/mocks"
	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type DeleteSuite struct {
	suite.Suite

	images    *mocks.MockOwnerImageStore
	builds    *mocks.MockBuildRepository
	keyboards *mocks.MockKeyboardRepository
	switches  *mocks.MockSwitchRepository
	ecParts   *mocks.MockECPartRepository
	sets      *mocks.MockKeycapSetRepository
	profiles  *mocks.MockProfileRepository
	notices   *admocks.MockNotifier
	account   accountdelete.Account
	logs      bytes.Buffer

	order []string
}

func TestDeleteSuite(t *testing.T) {
	suite.Run(t, new(DeleteSuite))
}

func (s *DeleteSuite) SetupTest() {
	s.images = mocks.NewMockOwnerImageStore(s.T())
	s.builds = mocks.NewMockBuildRepository(s.T())
	s.keyboards = mocks.NewMockKeyboardRepository(s.T())
	s.switches = mocks.NewMockSwitchRepository(s.T())
	s.ecParts = mocks.NewMockECPartRepository(s.T())
	s.sets = mocks.NewMockKeycapSetRepository(s.T())
	s.profiles = mocks.NewMockProfileRepository(s.T())
	s.notices = admocks.NewMockNotifier(s.T())
	s.account = accountdelete.Account{
		Images:     s.images,
		Builds:     s.builds,
		Keyboards:  s.keyboards,
		Switches:   s.switches,
		ECParts:    s.ecParts,
		KeycapSets: s.sets,
		Profiles:   s.profiles,
		Notices:    s.notices,
	}
	s.order = nil
	s.logs.Reset()
}

// ctx is alice's request context, logging to s.logs.
func (s *DeleteSuite) ctx() context.Context {
	ctx := apictx.WithUserID(s.T().Context(), "alice")
	return log.WithLogger(ctx, slog.New(slog.NewTextHandler(&s.logs, nil)))
}

func (s *DeleteSuite) expectAllDeleted() {
	s.builds.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.switches.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.ecParts.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.sets.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.images.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.profiles.EXPECT().Delete(mock.Anything).Return(nil)
}

func (s *DeleteSuite) record(step string) func(context.Context) {
	return func(context.Context) { s.order = append(s.order, step) }
}

func (s *DeleteSuite) TestDeletesTablesThenImagesThenProfile() {
	s.images.EXPECT().DeleteAll(mock.Anything).Run(s.record("images")).Return(nil)
	s.builds.EXPECT().DeleteAll(mock.Anything).Run(s.record("builds")).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Run(s.record("keyboards")).Return(nil)
	s.switches.EXPECT().DeleteAll(mock.Anything).Run(s.record("switches")).Return(nil)
	s.ecParts.EXPECT().DeleteAll(mock.Anything).Run(s.record("EC parts")).Return(nil)
	s.sets.EXPECT().DeleteAll(mock.Anything).Run(s.record("keycap sets")).Return(nil)
	s.profiles.EXPECT().Delete(mock.Anything).Run(s.record("profile")).Return(nil)
	s.notices.EXPECT().Notify(mock.Anything, "alice").
		Run(func(context.Context, string) { s.order = append(s.order, "notice") }).Return(nil)

	s.Require().NoError(s.account.Delete(s.ctx()))

	s.Equal([]string{"builds", "keyboards", "switches", "EC parts", "keycap sets", "images", "profile", "notice"}, s.order)
}

func (s *DeleteSuite) TestNoticeFails_LoggedNotReturned() {
	s.expectAllDeleted()
	s.notices.EXPECT().Notify(mock.Anything, "alice").Return(errors.New("sns down"))

	s.Require().NoError(s.account.Delete(s.ctx()))

	s.Contains(s.logs.String(), accountdelete.NoticeFailed)
	s.Contains(s.logs.String(), "owner_id=alice")
}

func (s *DeleteSuite) TestCallerGone_NoticeStillSent() {
	ctx, cancel := context.WithCancel(s.ctx())
	s.builds.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.switches.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.ecParts.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.sets.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.images.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.profiles.EXPECT().Delete(mock.Anything).Run(func(context.Context) { cancel() }).Return(nil)
	s.notices.EXPECT().
		Notify(mock.MatchedBy(func(ctx context.Context) bool { return ctx.Err() == nil }), "alice").
		Return(nil)

	s.Require().NoError(s.account.Delete(ctx))
}

func (s *DeleteSuite) TestNoNotifier_NoNotice() {
	s.expectAllDeleted()
	s.account.Notices = nil

	s.Require().NoError(s.account.Delete(s.ctx()))
}

func (s *DeleteSuite) TestImageDeleteFails_ProfileKept() {
	s.builds.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.switches.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.ecParts.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.sets.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.images.EXPECT().DeleteAll(mock.Anything).Return(errors.New("s3 down"))

	s.Require().Error(s.account.Delete(s.ctx()))
}

func (s *DeleteSuite) TestItemDeleteFails_ImagesAndProfileKept() {
	s.builds.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Return(errors.New("dynamo down"))

	s.Require().Error(s.account.Delete(s.ctx()))
}

func (s *DeleteSuite) TestProfileConflict_Matchable() {
	s.images.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.builds.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.keyboards.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.switches.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.ecParts.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.sets.EXPECT().DeleteAll(mock.Anything).Return(nil)
	s.profiles.EXPECT().Delete(mock.Anything).Return(repository.ErrMutationConflict)

	s.Require().ErrorIs(s.account.Delete(s.ctx()), repository.ErrMutationConflict)
	s.notices.AssertNotCalled(s.T(), "Notify", mock.Anything, mock.Anything)
}
