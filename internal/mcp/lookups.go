package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
)

var listLookupsTool = &mcp.Tool{
	Name:        "list_lookups",
	Description: "Lists every lookup category name (e.g. switch_type, vendor, keyboard_size). Call get_lookup with a category name to see its approved values.",
}

var getLookupTool = &mcp.Tool{
	Name:        "get_lookup",
	Description: "Returns the approved values for one lookup category.",
}

func handleListLookups() mcp.ToolHandlerFor[schema.ListLookupsInput, schema.ListLookupsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ schema.ListLookupsInput) (*mcp.CallToolResult, schema.ListLookupsOutput, error) {
		return nil, schema.ListLookupsOutput{Categories: lookup.ListCategoryNames(ctx)}, nil
	}
}

func handleGetLookup() mcp.ToolHandlerFor[schema.GetLookupInput, schema.GetLookupOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetLookupInput) (*mcp.CallToolResult, schema.GetLookupOutput, error) {
		l, ok := lookup.GetCategory(ctx, lookup.Category(in.Category))
		if !ok {
			return nil, schema.GetLookupOutput{}, fmt.Errorf("lookup category %q not found", in.Category)
		}

		return nil, repomcp.Lookup{}.ToMCP(l), nil
	}
}
