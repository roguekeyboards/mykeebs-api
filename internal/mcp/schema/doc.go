// Package schema holds MCP tool input/output types. Kept separate from
// [github.com/roguekeyboards/mykeebs-api/internal/handlers/api]'s generated REST
// types since mcp.AddTool infers tool schemas from jsonschema struct tags,
// which the generated types don't carry. Each input type's Validate method
// states the api/openapi.yaml rules REST's request validator would apply.
package schema
