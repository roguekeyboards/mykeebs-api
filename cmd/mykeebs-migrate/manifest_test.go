package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ManifestSuite struct {
	suite.Suite
}

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(ManifestSuite))
}

func (s *ManifestSuite) TestIDMap_IncrementalSaveAndReload() {
	dir := s.T().TempDir()

	m := newIDMap(dir, "user_target")
	m.Keyboards["kb-old"] = &mappedEntity{NewID: "kb-new", Images: map[string]string{"i-old": "i-new"}}
	s.Require().NoError(m.save())

	reloaded, err := loadOrNewIDMap(dir, "user_target")
	s.Require().NoError(err)
	s.Require().Contains(reloaded.Keyboards, "kb-old")
	s.Equal("kb-new", reloaded.Keyboards["kb-old"].NewID)
	s.Equal("i-new", reloaded.Keyboards["kb-old"].Images["i-old"])

	// A second entity added and saved onto the reloaded map persists too.
	reloaded.Switches["sw-old"] = &mappedEntity{NewID: "sw-new"}
	s.Require().NoError(reloaded.save())

	again, err := loadOrNewIDMap(dir, "user_target")
	s.Require().NoError(err)
	s.Contains(again.Keyboards, "kb-old")
	s.Contains(again.Switches, "sw-old")
}

func (s *ManifestSuite) TestLoadOrNewIDMap_FreshWhenAbsent() {
	dir := s.T().TempDir()
	m, err := loadOrNewIDMap(dir, "user_x")
	s.Require().NoError(err)
	s.Empty(m.Keyboards)
	s.Equal(filepath.Join(dir, "id-map.json"), m.path)
}

func (s *ManifestSuite) TestLoadOrNewIDMap_RejectsSubjectMismatch() {
	dir := s.T().TempDir()
	m := newIDMap(dir, "user_a")
	s.Require().NoError(m.save())

	_, err := loadOrNewIDMap(dir, "user_b")
	s.Require().Error(err)
	s.ErrorContains(err, "user_a")
}

func (s *ManifestSuite) TestWriteAndReadJSONFile_RoundTrips() {
	dir := s.T().TempDir()
	path := filepath.Join(dir, "manifest.json")
	in := dumpManifest{ToolVersion: "test", SourceSubject: "user_1", Counts: map[string]int{"keyboards": 3}}
	s.Require().NoError(writeJSONFile(path, in))

	var out dumpManifest
	s.Require().NoError(readJSONFile(path, &out))
	s.Equal("test", out.ToolVersion)
	s.Equal(3, out.Counts["keyboards"])
}

func (s *ManifestSuite) TestLoadOrNewIDMap_MapWithoutECParts_Loads() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "id-map.json"),
		[]byte(`{"target_subject":"user_x","switches":{"sw-old":{"new_id":"sw-new"}}}`), 0o600))

	m, err := loadOrNewIDMap(dir, "user_x")

	s.Require().NoError(err)
	s.NotNil(m.ECParts)
	s.Equal("sw-new", m.Switches["sw-old"].NewID)
}
