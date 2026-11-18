package gatewayroutes

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/suite"
	"go.yaml.in/yaml/v4"
)

type GatewayRoutesSuite struct {
	suite.Suite
}

func TestGatewayRoutesSuite(t *testing.T) {
	suite.Run(t, new(GatewayRoutesSuite))
}

const testSpec = `
openapi: 3.0.3
info: {title: t, version: "1"}
security:
  - BearerAuth: []
paths:
  /things:
    get:
      security:
        - {}
        - BearerAuth: []
      responses: {"200": {description: ok}}
    post:
      responses: {"201": {description: ok}}
  /lookups:
    get:
      security: []
      responses: {"200": {description: ok}}
components:
  securitySchemes:
    BearerAuth: {type: http, scheme: bearer}
`

func (s *GatewayRoutesSuite) loadSpec(doc string) *openapi3.T {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(doc))
	s.Require().NoError(err)
	return spec
}

func (s *GatewayRoutesSuite) TestRoutes_AuthorizesOnlyBearerRequiredOperations() {
	routes, err := Routes(s.loadSpec(testSpec))

	s.Require().NoError(err)
	s.Equal(append([]Route{
		{Method: "GET", Path: "/lookups"},
		{Method: "GET", Path: "/things"},
		{Method: "POST", Path: "/things", Authorized: true},
	}, extraRoutes...), routes)
}

func (s *GatewayRoutesSuite) TestRoutes_RejectsSchemeWithoutGatewayAuthorizer() {
	spec := s.loadSpec(`
openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /things:
    get:
      security:
        - ApiKey: []
      responses: {"200": {description: ok}}
components:
  securitySchemes:
    ApiKey: {type: apiKey, in: header, name: X-Key}
`)

	_, err := Routes(spec)

	s.ErrorContains(err, `GET /things: security scheme "ApiKey" has no gateway authorizer`)
}

func (s *GatewayRoutesSuite) TestDefinition_RendersRoutes() {
	body, err := Definition([]Route{
		{Method: "GET", Path: "/things"},
		{Method: "POST", Path: "/things", Authorized: true},
		{Method: "ANY", Path: "/mcp"},
	})
	s.Require().NoError(err)

	var doc struct {
		Paths map[string]map[string]struct {
			Integration map[string]any        `yaml:"x-amazon-apigateway-integration"`
			Security    []map[string][]string `yaml:"security"`
		} `yaml:"paths"`
		Components struct {
			SecuritySchemes map[string]any `yaml:"securitySchemes"`
		} `yaml:"components"`
	}
	s.Require().NoError(yaml.Unmarshal(body, &doc))

	s.Nil(doc.Paths["/things"]["get"].Security)
	s.Equal([]map[string][]string{{authorizerName: {}}}, doc.Paths["/things"]["post"].Security)
	s.Equal("aws_proxy", doc.Paths["/things"]["post"].Integration["type"])
	s.Contains(doc.Paths["/mcp"], "x-amazon-apigateway-any-method")
	s.Contains(doc.Components.SecuritySchemes, authorizerName)
}

func (s *GatewayRoutesSuite) TestDefinition_RejectsDuplicateRoute() {
	_, err := Definition([]Route{
		{Method: "GET", Path: "/things"},
		{Method: "GET", Path: "/things", Authorized: true},
	})

	s.ErrorContains(err, "duplicate route GET /things")
}
