package main

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
)

type VerifySuite struct {
	suite.Suite
}

func TestVerifySuite(t *testing.T) {
	suite.Run(t, new(VerifySuite))
}

func (s *VerifySuite) TestCompareScalars_CurrencyDiffers_Matches() {
	dump := `{"id":"a","brand":"B","purchase":{"price":10,"currency":"EUR"},"stabs":{"price":5,"currency":"EUR"},"currency":"EUR"}`
	live := `{"id":"b","brand":"B","purchase":{"price":10,"currency":"USD"},"stabs":{"price":5,"currency":"USD"},"currency":"USD"}`

	ok, detail := compareScalars([]byte(dump), []byte(live))

	s.True(ok, detail)
}

func (s *VerifySuite) TestCompareScalars_DumpPredatesCurrency_Matches() {
	dump := `{"brand":"B","kits":[{"name":"Base","purchase":{"price":10}}]}`
	live := `{"brand":"B","kits":[{"name":"Base","purchase":{"price":10,"currency":"USD"}}]}`

	ok, detail := compareScalars([]byte(dump), []byte(live))

	s.True(ok, detail)
}

func (s *VerifySuite) TestCompareScalars_PriceDiffers_Mismatch() {
	dump := `{"brand":"B","purchase":{"price":10,"currency":"USD"}}`
	live := `{"brand":"B","purchase":{"price":12,"currency":"USD"}}`

	ok, _ := compareScalars([]byte(dump), []byte(live))

	s.False(ok)
}

func (s *VerifySuite) TestCompareKeyboards_LegacyDumpMatchesUpgradedLive() {
	dump := `{"id":"a","brand":"B","purchase":{"price":10},` +
		`"design":{"top_case":{"material":"Aluminum"},"plates":["AL","AL","PC"]},"pcb":{"firmware":"QMK/VIA"}}`
	live := `{"id":"b","brand":"B","purchase":{"price":10,"currency":"USD"},"total_cost":10,"currency":"USD",` +
		`"design":{"top_case":{"material":"Aluminum"}},` +
		`"plates":[{"id":"n1","material":"AL"},{"id":"n2","material":"AL"},{"id":"n3","material":"PC"}],` +
		`"pcbs":[{"id":"n4","firmware":"QMK/VIA"}]}`

	ok, detail := compareKeyboards([]byte(dump), []byte(live))

	s.True(ok, detail)
}

func (s *VerifySuite) TestCompareKeyboards_PartDiffers_Mismatch() {
	dump := `{"brand":"B","plates":[{"id":"p1","material":"AL","color":"Black"}]}`
	live := `{"brand":"B","plates":[{"id":"n1","material":"AL","color":"Silver"}]}`

	ok, _ := compareKeyboards([]byte(dump), []byte(live))

	s.False(ok)
}

func (s *VerifySuite) TestComparePartRef() {
	ids := map[string]string{"p-old": "p-new"}

	s.Empty(comparePartRef("plate", nil, nil, ids))
	s.Empty(comparePartRef("plate", new("p-old"), new("p-new"), ids))
	s.NotEmpty(comparePartRef("plate", new("p-old"), new("other"), ids))
	s.NotEmpty(comparePartRef("plate", new("p-old"), nil, ids))
	s.NotEmpty(comparePartRef("plate", nil, new("p-new"), ids))
}

func (s *VerifySuite) ecBuildMap() *idMap {
	return &idMap{
		Keyboards: map[string]*mappedEntity{"kb-old": {NewID: "kb-new"}},
		ECParts:   map[string]*mappedEntity{"ec-old": {NewID: "ec-new"}},
	}
}

func (s *VerifySuite) TestCompareBuildRefs_ECPartsRemapped_Matches() {
	dump := api.Build{
		BuildType: "EC",
		Keyboard:  api.BuildKeyboardRef{Id: "kb-old"},
		EcParts:   &[]api.BuildECPartEntryResolved{{Count: 87, EcPart: api.BuildECPartRef{Id: "ec-old"}}},
	}
	live := api.Build{
		BuildType: "EC",
		Keyboard:  api.BuildKeyboardRef{Id: "kb-new"},
		EcParts:   &[]api.BuildECPartEntryResolved{{Count: 87, EcPart: api.BuildECPartRef{Id: "ec-new"}}},
	}

	s.Empty(compareBuildRefs(dump, live, s.ecBuildMap()))
}

func (s *VerifySuite) TestCompareBuildRefs_ECPartCountDiffers_Mismatch() {
	dump := api.Build{
		BuildType: "EC",
		Keyboard:  api.BuildKeyboardRef{Id: "kb-old"},
		EcParts:   &[]api.BuildECPartEntryResolved{{Count: 87, EcPart: api.BuildECPartRef{Id: "ec-old"}}},
	}
	live := api.Build{
		BuildType: "EC",
		Keyboard:  api.BuildKeyboardRef{Id: "kb-new"},
		EcParts:   &[]api.BuildECPartEntryResolved{{Count: 60, EcPart: api.BuildECPartRef{Id: "ec-new"}}},
	}

	s.Contains(compareBuildRefs(dump, live, s.ecBuildMap()), "ec_part[0] count")
}

func (s *VerifySuite) TestCompareBuildRefs_DumpPredatesBuildType_MatchesMX() {
	dump := api.Build{Keyboard: api.BuildKeyboardRef{Id: "kb-old"}}
	live := api.Build{BuildType: "MX", Keyboard: api.BuildKeyboardRef{Id: "kb-new"}}

	s.Empty(compareBuildRefs(dump, live, s.ecBuildMap()))
}

func (s *VerifySuite) TestCompareBuildRefs_BuildTypeDiffers_Mismatch() {
	dump := api.Build{BuildType: "EC", Keyboard: api.BuildKeyboardRef{Id: "kb-old"}}
	live := api.Build{BuildType: "MX", Keyboard: api.BuildKeyboardRef{Id: "kb-new"}}

	s.Contains(compareBuildRefs(dump, live, s.ecBuildMap()), "build_type")
}
