package cascadedelete_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/cascadedelete"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/mocks"
)

type DeleteECPartSuite struct {
	suite.Suite

	mockECParts      *mocks.MockECPartRepository
	mockBuilds       *mocks.MockBuildRepository
	mockBuildImages  *mocks.MockBuildImageStore
	mockECPartImages *mocks.MockECPartImageStore
	ctx              context.Context
}

func TestDeleteECPartSuite(t *testing.T) {
	suite.Run(t, new(DeleteECPartSuite))
}

func (s *DeleteECPartSuite) SetupTest() {
	s.mockECParts = mocks.NewMockECPartRepository(s.T())
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockBuildImages = mocks.NewMockBuildImageStore(s.T())
	s.mockECPartImages = mocks.NewMockECPartImageStore(s.T())
	s.ctx = s.T().Context()
}

func (s *DeleteECPartSuite) TestBlock_NoReferencingBuilds_DeletesECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return(nil, nil)
	s.mockECParts.EXPECT().
		Get(s.ctx, "alice", "p1").
		Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockECParts.EXPECT().
		Delete(s.ctx, "p1").
		Return(nil)

	result, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteECPartSuite) TestBlock_ECPartHadImage_DeletesFromS3BeforeDB() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return(nil, nil)
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/up1")
	s.mockECParts.EXPECT().
		Get(s.ctx, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", ImagePath: &key}, nil)
	s.mockECPartImages.EXPECT().
		Delete(s.ctx, key).
		Return(nil)
	s.mockECParts.EXPECT().
		Delete(s.ctx, "p1").
		Return(nil)

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteBlock,
	)

	s.Require().NoError(err)
}

func (s *DeleteECPartSuite) TestBlock_ImageDeleteFails_ReturnsErrorWithoutDeletingECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return(nil, nil)
	key := repository.ECPartImageKey("ec-parts/alice/p1/image/up1")
	s.mockECParts.EXPECT().
		Get(s.ctx, "alice", "p1").
		Return(&repository.ECPart{ID: "p1", ImagePath: &key}, nil)
	s.mockECPartImages.EXPECT().
		Delete(s.ctx, key).
		Return(errors.New("s3 unavailable"))

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
}

func (s *DeleteECPartSuite) TestBlock_ReferencingBuilds_ReturnsBlockedError_DeletesNothing() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return([]string{"build-1", "build-2"}, nil)

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteBlock,
	)

	s.Require().ErrorIs(err, cascadedelete.ErrBlocked)

	var blocked *cascadedelete.BlockedError
	s.Require().ErrorAs(err, &blocked)
	s.ElementsMatch([]string{"build-1", "build-2"}, blocked.BuildIDs)

}

func (s *DeleteECPartSuite) TestCascade_NoReferencingBuilds_DeletesOnlyECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return(nil, nil)
	s.mockECParts.EXPECT().
		Get(s.ctx, "alice", "p1").
		Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockECParts.EXPECT().
		Delete(s.ctx, "p1").
		Return(nil)

	result, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.Empty(result.DeletedBuildIDs)
}

func (s *DeleteECPartSuite) TestCascade_ReferencingBuilds_DeletesEachBuildImagesThenBuildThenECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return([]string{"build-1", "build-2"}, nil)
	s.mockBuilds.EXPECT().
		Get(s.ctx, "alice", "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: "builds/alice/build-1/images/img1"},
		})}, nil)
	s.mockBuildImages.EXPECT().
		DeleteBuildImage(s.ctx, repository.BuildImageKey("builds/alice/build-1/images/img1")).
		Return(nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(nil)
	s.mockBuilds.EXPECT().
		Get(s.ctx, "alice", "build-2").
		Return(&repository.Build{ID: "build-2"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-2").
		Return(nil)
	s.mockECParts.EXPECT().
		Get(s.ctx, "alice", "p1").
		Return(&repository.ECPart{ID: "p1"}, nil)
	s.mockECParts.EXPECT().
		Delete(s.ctx, "p1").
		Return(nil)

	result, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteCascade,
	)

	s.Require().NoError(err)
	s.ElementsMatch([]string{"build-1", "build-2"}, result.DeletedBuildIDs)
}

func (s *DeleteECPartSuite) TestCascade_BuildImageDeleteFails_ReturnsErrorWithoutDeletingBuildOrECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(s.ctx, "alice", "build-1").
		Return(&repository.Build{ID: "build-1", Images: repository.BuildImagesMap([]repository.BuildImage{
			{ImageID: "img1", Path: "builds/alice/build-1/images/img1"},
		})}, nil)
	s.mockBuildImages.EXPECT().
		DeleteBuildImage(s.ctx, repository.BuildImageKey("builds/alice/build-1/images/img1")).
		Return(errors.New("s3 unavailable"))

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
}

func (s *DeleteECPartSuite) TestCascade_BuildDeleteFails_ReturnsErrorWithoutDeletingECPart() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return([]string{"build-1"}, nil)
	s.mockBuilds.EXPECT().
		Get(s.ctx, "alice", "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockBuilds.EXPECT().
		Delete(s.ctx, "build-1").
		Return(errors.New("dynamo unavailable"))

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteCascade,
	)

	s.Require().Error(err)
}

func (s *DeleteECPartSuite) TestUnknownOnDelete_ReturnsError_CallsNothing() {
	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDelete("bogus"),
	)

	s.Require().Error(err)
}

func (s *DeleteECPartSuite) TestFindReferencingBuildsFails_ReturnsError() {
	s.mockBuilds.EXPECT().
		FindBuildsReferencingECPart(s.ctx, "alice", "p1").
		Return(nil, errors.New("query failed"))

	_, err := cascadedelete.DeleteECPart(
		s.ctx, s.mockECParts, s.mockBuilds, s.mockBuildImages, s.mockECPartImages,
		"alice", "p1", cascadedelete.OnDeleteBlock,
	)

	s.Require().Error(err)
	s.Require().NotErrorIs(err, cascadedelete.ErrBlocked)
}
