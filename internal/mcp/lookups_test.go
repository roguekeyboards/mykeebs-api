package mcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
)

type HandleListLookupsSuite struct {
	suite.Suite
}

func TestHandleListLookupsSuite(t *testing.T) {
	suite.Run(t, new(HandleListLookupsSuite))
}

func (s *HandleListLookupsSuite) TestSucceeds() {
	handler := handleListLookups()
	result, out, err := handler(s.T().Context(), nil, schema.ListLookupsInput{})

	s.Require().NoError(err)
	s.Nil(result)
	s.Contains(out.Categories, "vendor")
	s.Contains(out.Categories, "switch_type")
}

type HandleGetLookupSuite struct {
	suite.Suite
}

func TestHandleGetLookupSuite(t *testing.T) {
	suite.Run(t, new(HandleGetLookupSuite))
}

func (s *HandleGetLookupSuite) TestSucceeds() {
	handler := handleGetLookup()
	result, out, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "vendor"})

	s.Require().NoError(err)
	s.Nil(result)
	s.Equal("vendor", out.Category)
	s.Contains(out.Values, "Amazon")
}

func (s *HandleGetLookupSuite) TestNotFound_ReturnsError() {
	handler := handleGetLookup()
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "not-a-real-category"})

	s.Require().ErrorContains(err, `"not-a-real-category" not found`)
}

func (s *HandleGetLookupSuite) TestKeyboardLayout_ReturnsTypedValues() {
	handler := handleGetLookup()
	_, out, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: string(lookup.CategoryKeyboardLayout)})

	s.Require().NoError(err)
	s.Equal(string(lookup.CategoryKeyboardLayout), out.Category)
	s.NotEmpty(out.Values)
}
