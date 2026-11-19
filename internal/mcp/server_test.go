package mcp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/auth"
	authmocks "github.com/roguekeyboards/mykeebs-api/internal/auth/mocks"
)

type NewSuite struct {
	suite.Suite
	srv *httptest.Server
}

func TestNewSuite(t *testing.T) {
	suite.Run(t, new(NewSuite))
}

func (s *NewSuite) SetupTest() {
	verifier := authmocks.NewMockTokenVerifier(s.T())
	verifier.EXPECT().Verify(mock.Anything, "token").Return(&oidc.IDToken{
		Subject:  callerID,
		Audience: []string{"aud"},
		Expiry:   time.Now().Add(time.Hour),
	}, nil).Maybe()

	h := New(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, accountdelete.Account{}, nil,
		auth.NewVerifierForTesting(verifier, "aud"), "https://issuer.example", "test")
	s.srv = httptest.NewServer(h.Streamable)
	s.T().Cleanup(s.srv.Close)
}

// bearer adds the token the mock verifier accepts.
type bearer struct{}

func (bearer) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer token")
	return http.DefaultTransport.RoundTrip(r)
}

func (s *NewSuite) TestAdvertisesToolsWithoutListChanged() {
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "client", Version: "test"}, nil)
	session, err := client.Connect(s.T().Context(), &sdkmcp.StreamableClientTransport{
		Endpoint:   s.srv.URL,
		HTTPClient: &http.Client{Transport: bearer{}},
	}, nil)
	s.Require().NoError(err)
	defer func() { _ = session.Close() }()

	tools := session.InitializeResult().Capabilities.Tools
	s.Require().NotNil(tools)
	s.False(tools.ListChanged)
}

// A held-open subscriptions/listen can only end in Lambda's timeout, so even
// a client that ignores the capabilities and asks for tool changes must get
// an immediate answer.
func (s *NewSuite) TestSubscriptionsListenReturnsImmediately() {
	body := `{"jsonrpc":"2.0","id":1,"method":"subscriptions/listen","params":{` +
		`"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientInfo":{"name":"client","version":"test"},` +
		`"io.modelcontextprotocol/clientCapabilities":{}},` +
		`"notifications":{"toolsListChanged":true}}}`
	ctx, cancel := context.WithTimeout(s.T().Context(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.srv.URL, bytes.NewBufferString(body))
	s.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "subscriptions/listen")

	resp, err := (&http.Client{Transport: bearer{}}).Do(req)
	s.Require().NoError(err)
	defer func() { _ = resp.Body.Close() }()
	got, err := io.ReadAll(resp.Body)

	s.Require().NoError(err)
	s.Equal(http.StatusOK, resp.StatusCode, string(got))
	s.Contains(string(got), `"result"`)
}
