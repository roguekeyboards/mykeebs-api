package mcp

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
)

type AddToolSuite struct {
	suite.Suite
}

func TestAddToolSuite(t *testing.T) {
	suite.Run(t, new(AddToolSuite))
}

type addToolInput struct {
	Name string `json:"name"`
}

func (in addToolInput) Validate(v *validate.V) {
	v.NotBlank("name", in.Name)
}

func (s *AddToolSuite) call(name string) (*sdkmcp.CallToolResult, bool) {
	ctx := s.T().Context()
	called := false

	server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "test"}, nil)
	addTool(server, &sdkmcp.Tool{Name: "named"}, func(context.Context, *sdkmcp.CallToolRequest, addToolInput) (*sdkmcp.CallToolResult, struct{}, error) {
		called = true
		return nil, struct{}{}, nil
	})

	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = serverSession.Close() })

	session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "client"}, nil).Connect(ctx, clientTransport, nil)
	s.Require().NoError(err)
	s.T().Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "named", Arguments: map[string]any{"name": name}})
	s.Require().NoError(err)

	return result, called
}

func (s *AddToolSuite) TestValidInput_RunsTheHandler() {
	result, called := s.call("Gateron")

	s.False(result.IsError)
	s.True(called)
}

func (s *AddToolSuite) TestInvalidInput_ReturnsToolErrorWithoutRunningTheHandler() {
	result, called := s.call(" ")

	s.True(result.IsError)
	s.False(called)
	s.Require().Len(result.Content, 1)
	s.Equal("name: must not be blank", result.Content[0].(*sdkmcp.TextContent).Text)
}
