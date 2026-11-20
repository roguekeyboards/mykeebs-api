package lookup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
)

type CatalogSuite struct {
	suite.Suite
}

func TestCatalogSuite(t *testing.T) {
	suite.Run(t, new(CatalogSuite))
}

func (s *CatalogSuite) TestEveryCategory_HasNonEmptyValues() {
	ctx := context.Background()

	for _, category := range lookup.ListCategories(ctx) {
		l, ok := lookup.GetCategory(ctx, category)
		s.Require().True(ok, "category %q from ListCategories not found via GetCategory", category)

		s.NotEmpty(l.Values, "category %q has no values", category)
	}
}

func (s *CatalogSuite) TestListCategoryNames_MatchesListCategoriesWidened() {
	ctx := context.Background()

	categories := lookup.ListCategories(ctx)
	names := lookup.ListCategoryNames(ctx)

	s.Require().Len(names, len(categories))
	for i, c := range categories {
		s.Equal(string(c), names[i])
	}
}

func (s *CatalogSuite) TestListCategoryNames_MutatingResult_DoesNotAffectLaterCalls() {
	ctx := context.Background()

	first := lookup.ListCategoryNames(ctx)
	original := first[0]
	first[0] = "corrupted"

	second := lookup.ListCategoryNames(ctx)
	s.Equal(original, second[0])
}

func (s *CatalogSuite) TestGetCategory_MutatingValues_DoesNotAffectLaterCalls() {
	ctx := context.Background()

	first, ok := lookup.GetCategory(ctx, lookup.CategoryVendor)
	s.Require().True(ok)
	original := first.Values[0]
	first.Values[0] = "corrupted"

	second, ok := lookup.GetCategory(ctx, lookup.CategoryVendor)
	s.Require().True(ok)
	s.Equal(original, second.Values[0])
}

func (s *CatalogSuite) TestGetCategory_UnknownCategory_ReturnsFalse() {
	_, ok := lookup.GetCategory(context.Background(), "not-a-real-category")
	s.False(ok)
}

func (s *CatalogSuite) TestKeyboardSize_IncludesNumpad() {
	l, ok := lookup.GetCategory(s.T().Context(), lookup.CategoryKeyboardSize)
	s.Require().True(ok)

	s.Contains(l.Strings(), "Numpad")
}

func (s *CatalogSuite) TestKeyboardLayout_IncludesNumpadRowLayouts() {
	l, ok := lookup.GetCategory(s.T().Context(), lookup.CategoryKeyboardLayout)
	s.Require().True(ok)

	s.Subset(l.LayoutValues(), []lookup.LayoutValue{
		{Name: "5 Row", Sizes: []string{"Numpad"}},
		{Name: "6 Row", Sizes: []string{"Numpad"}},
	})
}

func (s *CatalogSuite) TestSwitchType_ExcludesEC() {
	l, ok := lookup.GetCategory(s.T().Context(), lookup.CategorySwitchType)
	s.Require().True(ok)

	s.NotContains(l.Strings(), "EC")
}
