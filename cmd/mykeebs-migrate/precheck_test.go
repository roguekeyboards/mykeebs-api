package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

type PrecheckSuite struct {
	suite.Suite

	dumpDir string
}

func TestPrecheckSuite(t *testing.T) {
	suite.Run(t, new(PrecheckSuite))
}

func (s *PrecheckSuite) SetupTest() {
	s.dumpDir = s.T().TempDir()
}

// writeImage creates dir/name with size bytes (sparse, so large sizes are
// cheap) and returns its manifest entry.
func (s *PrecheckSuite) writeImage(dir, name string, size int64) imageManifestEntry {
	s.Require().NoError(os.MkdirAll(dir, 0o750))
	f, err := os.Create(filepath.Join(dir, name)) //nolint:gosec // test temp dir.
	s.Require().NoError(err)
	s.Require().NoError(f.Truncate(size))
	s.Require().NoError(f.Close())
	return imageManifestEntry{Filename: name, ContentType: "image/webp", Bytes: int(size)}
}

func (s *PrecheckSuite) writeJSON(path string, v any) {
	data, err := json.Marshal(v)
	s.Require().NoError(err)
	s.Require().NoError(os.WriteFile(path, data, 0o600))
}

// arrayItem writes sub/id/images with one image per size.
func (s *PrecheckSuite) arrayItem(sub, id string, sizes ...int64) {
	imagesDir := filepath.Join(s.dumpDir, sub, id, "images")
	manifest := make([]imageManifestEntry, len(sizes))
	for i, size := range sizes {
		manifest[i] = s.writeImage(imagesDir, fmt.Sprintf("img%d.webp", i), size)
	}
	s.writeJSON(filepath.Join(imagesDir, "images.manifest.json"), manifest)
}

// singleSlot writes dir/image.webp of size plus its manifest.
func (s *PrecheckSuite) singleSlot(dir string, size int64) {
	s.writeJSON(filepath.Join(dir, "image.manifest.json"), s.writeImage(dir, "image.webp", size))
}

// sizes returns n copies of size.
func sizes(n int, size int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = size
	}
	return out
}

func (s *PrecheckSuite) TestWithinLimits_Passes() {
	s.arrayItem("keyboards", "kb1", 100, repository.MaxImageSizeBytes)
	s.arrayItem("builds", "b1", sizes(repository.MaxImagesPerItem, 100)...)
	s.singleSlot(filepath.Join(s.dumpDir, "switches", "sw1"), 100)
	s.singleSlot(filepath.Join(s.dumpDir, "ec-parts", "ec1"), 100)
	s.singleSlot(filepath.Join(s.dumpDir, "keycap-sets", "ks1", "kits", "kit1"), 100)
	s.Require().NoError(os.MkdirAll(filepath.Join(s.dumpDir, "keyboards", "no-images"), 0o750))

	s.Require().NoError(checkImageLimits(s.dumpDir))
}

func (s *PrecheckSuite) TestEmptyDump_Passes() {
	s.Require().NoError(checkImageLimits(s.dumpDir))
}

func (s *PrecheckSuite) TestReportsEveryProblem() {
	s.arrayItem("keyboards", "kb1", repository.MaxImageSizeBytes+1)
	s.arrayItem("builds", "b1", sizes(repository.MaxImagesPerItem+1, 100)...)
	s.singleSlot(filepath.Join(s.dumpDir, "switches", "sw1"), 0)
	s.singleSlot(filepath.Join(s.dumpDir, "ec-parts", "ec1"), repository.MaxImageSizeBytes+1)
	s.singleSlot(filepath.Join(s.dumpDir, "keycap-sets", "ks1", "kits", "kit1"), repository.MaxImageSizeBytes+1)
	avatarDir := filepath.Join(s.dumpDir, profileDir)
	s.writeJSON(filepath.Join(avatarDir, "avatar.manifest.json"), s.writeImage(avatarDir, "avatar.webp", 0))

	err := checkImageLimits(s.dumpDir)

	s.Require().ErrorContains(err, filepath.Join(profileDir, "avatar.webp"))
	s.Require().ErrorContains(err, filepath.Join("keyboards", "kb1", "images", "img0.webp"))
	s.Require().ErrorContains(err, "builds/b1 has 11 images")
	s.Require().ErrorContains(err, filepath.Join("switches", "sw1", "image.webp"))
	s.Require().ErrorContains(err, filepath.Join("ec-parts", "ec1", "image.webp"))
	s.Require().ErrorContains(err, filepath.Join("kits", "kit1", "image.webp"))
}

func (s *PrecheckSuite) TestNonWebP_Reported() {
	dir := filepath.Join(s.dumpDir, "switches", "sw1")
	entry := s.writeImage(dir, "image.png", 100)
	entry.ContentType = "image/png"
	s.writeJSON(filepath.Join(dir, "image.manifest.json"), entry)

	s.Require().ErrorContains(checkImageLimits(s.dumpDir), "image/png")
}

func (s *PrecheckSuite) TestMissingImageFile_IsError() {
	imagesDir := filepath.Join(s.dumpDir, "keyboards", "kb1", "images")
	s.Require().NoError(os.MkdirAll(imagesDir, 0o750))
	s.writeJSON(filepath.Join(imagesDir, "images.manifest.json"), []imageManifestEntry{{Filename: "gone.webp"}})

	s.Require().ErrorContains(checkImageLimits(s.dumpDir), "gone.webp")
}
