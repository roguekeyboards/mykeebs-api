// Package gatewayroutes builds the HTTP API's OpenAPI definition, the routes
// API Gateway sends to ApiFunction, from api/openapi.yaml plus the few routes
// outside it. template.yaml pulls the result in with AWS::Include rather than
// declaring one SAM event per route: each event adds its own statement to
// ApiFunction's resource policy, and at around 50 routes the policy outgrew
// Lambda's 20 KB limit. With the definition written out, one wildcard
// permission (ApiFunctionInvokePermission) covers every route.
package gatewayroutes

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v4"
)

// Route is one route API Gateway forwards to ApiFunction.
type Route struct {
	// Method is an HTTP method, or "ANY" for every method.
	Method string
	Path   string
	// Authorized routes go through the gateway's JWT authorizer.
	Authorized bool
}

// extraRoutes are the routes api/openapi.yaml doesn't cover. None has a
// gateway authorizer.
var extraRoutes = []Route{
	// /mcp verifies tokens itself (middleware.RequireAuth) so a 401 can carry
	// the WWW-Authenticate header the MCP spec requires, which API Gateway's
	// own rejection can't include.
	{Method: "ANY", Path: "/mcp"},
	// This metadata tells an unauthenticated client where to authenticate.
	{Method: http.MethodGet, Path: "/.well-known/oauth-protected-resource"},
	{Method: http.MethodGet, Path: "/.well-known/oauth-protected-resource/mcp"},
	// ANY so a non-GET request reaches the handler's own 405 instead of a 404.
	{Method: "ANY", Path: "/authorize"},
	// Redirects to the pages' new home on mykeebs.com.
	{Method: "ANY", Path: "/privacy"},
	{Method: "ANY", Path: "/terms"},
	{Method: "ANY", Path: "/help/mcp"},
}

const bearerScheme = "BearerAuth"

// Routes returns every route: one per operation in spec, then extraRoutes.
// An operation whose security allows anonymous callers ([] or a {} entry)
// gets no gateway authorizer; the app applies its own optional auth.
func Routes(spec *openapi3.T) ([]Route, error) {
	var routes []Route
	for _, path := range slices.Sorted(maps.Keys(spec.Paths.Map())) {
		ops := spec.Paths.Value(path).Operations()
		for _, m := range slices.Sorted(maps.Keys(ops)) {
			op := ops[m]
			security := spec.Security
			if op.Security != nil {
				security = *op.Security
			}
			authorized, err := requiresBearer(security)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", m, path, err)
			}
			routes = append(routes, Route{Method: m, Path: path, Authorized: authorized})
		}
	}
	return append(routes, extraRoutes...), nil
}

func requiresBearer(security openapi3.SecurityRequirements) (bool, error) {
	if len(security) == 0 {
		return false, nil
	}
	bearer := false
	for _, req := range security {
		if len(req) == 0 {
			return false, nil
		}
		for scheme := range req {
			if scheme != bearerScheme {
				return false, fmt.Errorf("security scheme %q has no gateway authorizer", scheme)
			}
			bearer = true
		}
	}
	return bearer, nil
}

const authorizerName = "OidcAuthorizer"

type integration struct {
	Type                 string            `yaml:"type"`
	HTTPMethod           string            `yaml:"httpMethod"`
	PayloadFormatVersion string            `yaml:"payloadFormatVersion"`
	URI                  map[string]string `yaml:"uri"`
}

type operation struct {
	Integration integration           `yaml:"x-amazon-apigateway-integration"`
	Security    []map[string][]string `yaml:"security,omitempty"`
}

var lambdaIntegration = integration{
	Type:                 "aws_proxy",
	HTTPMethod:           http.MethodPost,
	PayloadFormatVersion: "2.0",
	URI: map[string]string{
		"Fn::Sub": "arn:${AWS::Partition}:apigateway:${AWS::Region}:lambda:path/2015-03-31/functions/${ApiFunction.Arn}/invocations",
	},
}

// The authorizer validates the same issuer and audience as the app's own
// verifier, from template.yaml's parameters.
var securitySchemes = map[string]any{
	authorizerName: map[string]any{
		"type": "oauth2",
		"x-amazon-apigateway-authorizer": map[string]any{
			"type":           "jwt",
			"identitySource": "$request.header.Authorization",
			"jwtConfiguration": map[string]any{
				"issuer":   map[string]string{"Ref": "OidcIssuerBaseUrl"},
				"audience": []any{map[string]string{"Ref": "OidcAudience"}},
			},
		},
	},
}

// Definition renders routes as the HTTP API's OpenAPI body, in
// CloudFormation's long-form intrinsics (AWS::Include doesn't take YAML
// shorthand like !Sub).
func Definition(routes []Route) ([]byte, error) {
	paths := map[string]map[string]operation{}
	for _, r := range routes {
		op := operation{Integration: lambdaIntegration}
		if r.Authorized {
			op.Security = []map[string][]string{{authorizerName: {}}}
		}
		key := "x-amazon-apigateway-any-method"
		if r.Method != "ANY" {
			key = strings.ToLower(r.Method)
		}
		if paths[r.Path] == nil {
			paths[r.Path] = map[string]operation{}
		}
		if _, dup := paths[r.Path][key]; dup {
			return nil, fmt.Errorf("duplicate route %s %s", r.Method, r.Path)
		}
		paths[r.Path][key] = op
	}

	doc := struct {
		OpenAPI    string                          `yaml:"openapi"`
		Info       map[string]any                  `yaml:"info"`
		Paths      map[string]map[string]operation `yaml:"paths"`
		Components map[string]any                  `yaml:"components"`
	}{
		OpenAPI:    "3.0.1",
		Info:       map[string]any{"title": map[string]string{"Ref": "AWS::StackName"}, "version": "1.0"},
		Paths:      paths,
		Components: map[string]any{"securitySchemes": securitySchemes},
	}
	var buf bytes.Buffer
	buf.WriteString("# Code generated by api/gen from api/openapi.yaml. DO NOT EDIT.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
