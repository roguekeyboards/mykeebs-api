package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/auth"
	ctxpkg "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	logpkg "github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/middleware"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// Handlers holds the HTTP handlers the router needs to mount: the MCP
// Streamable HTTP endpoint itself, and the RFC 9728 Protected Resource
// Metadata handler (served at two paths - see MetadataPath/RootMetadataPath
// below). Go's net/http.ServeMux won't route requests for the well-known
// metadata paths to the streamable handler just because it's registered at
// "/mcp" (an exact-match mux pattern shadows it) — all three routes must be
// mounted explicitly.
//
// The metadata handler builds its "resource" URL from each request's Host
// header rather than a static, deploy-time-known value. This isn't just a
// workaround: referencing the API Gateway's own generated domain from a
// CloudFormation env var on this same Lambda creates a genuine circular
// dependency (the function's Events block makes the API depend on the
// function; a !Sub referencing the API's logical ID would make the function
// depend back on the API). Deriving it from the request is also more
// correct for RFC 9728 — the "resource" identifier should reflect whatever
// URL the client actually used to reach the server, which stays right even
// if a custom domain is added later.
type Handlers struct {
	Streamable http.Handler
	// MetadataPath and RootMetadataPath both serve the identical metadata
	// document. RFC 9728 discovery checks the endpoint-scoped path first,
	// falling back to the origin root - a client that only found the root
	// document would see a "resource" field naming /mcp while having
	// fetched the document from a different path, which a strict client
	// may treat as a mismatch. Serving both keeps the document's own
	// "resource" claim consistent with wherever a client actually found it.
	MetadataPath     string
	RootMetadataPath string
	Metadata         http.Handler
}

// MetadataPath and RootMetadataPath serve the same RFC 9728 metadata
// document; see Handlers for why both are served.
const (
	MetadataPath     = "/.well-known/oauth-protected-resource/mcp"
	RootMetadataPath = "/.well-known/oauth-protected-resource"
)

// New builds the MCP server. issuerURL is the OIDC issuer MCP clients should
// authenticate against, advertised via RFC 9728 Protected Resource
// Metadata. verifier authenticates every /mcp request in-process (see
// middleware.RequireAuth for why /mcp can't rely on API Gateway's native
// authorizer the way REST's required-auth routes do). version is
// advertised to MCP clients on connect.
func New(
	switchRepo repository.SwitchRepository,
	switchImageStore repository.SwitchImageStore,
	ecPartRepo repository.ECPartRepository,
	ecPartImageStore repository.ECPartImageStore,
	keyboardRepo repository.KeyboardRepository,
	keyboardImageStore repository.KeyboardImageStore,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
	buildImageStore repository.BuildImageStore,
	profileRepo repository.ProfileRepository,
	profileImageStore repository.ProfileImageStore,
	account accountdelete.Account,
	uploads *uploadlimit.Limiter,
	verifier *auth.Verifier,
	issuerURL, version string,
) Handlers {
	// No listChanged: the SDK's default advertises it, inviting clients to
	// open a subscriptions/listen POST the server holds open, and Lambda
	// buffers responses, so that POST can only end in a timeout. The tool
	// list never changes at runtime anyway.
	mcpServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "mykeebs-api", Version: version}, &sdkmcp.ServerOptions{
		Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{}},
	})
	mcpServer.AddReceivingMiddleware(identityMiddleware())

	registerTools(mcpServer, switchRepo, switchImageStore, ecPartRepo, ecPartImageStore, keyboardRepo, keyboardImageStore, keycapSetRepo, imageStore, buildRepo, buildImageStore, profileRepo, profileImageStore, account, uploads)

	streamable := sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return mcpServer },
		&sdkmcp.StreamableHTTPOptions{
			Stateless: true,
			// aws-lambda-web-adapter proxies over 127.0.0.1 with the real
			// Host header intact, which DNS-rebinding protection can't
			// distinguish from an actual rebinding attack - safe to disable
			// here since that attack requires a browser, not a Lambda proxy.
			DisableLocalhostProtection: true,
		},
	)

	return Handlers{
		Streamable:       middleware.RequireAuth(verifier, MetadataPath)(streamable),
		MetadataPath:     MetadataPath,
		RootMetadataPath: RootMetadataPath,
		Metadata:         metadataHandler(issuerURL),
	}
}

// errNoTokenInfo guards against a request ever reaching the tool dispatch
// layer without middleware.RequireAuth having run first (see New, which
// wraps the streamable handler with it) - unreachable today since it's the
// only entrypoint into this server, but this fails closed rather than
// silently proceeding unauthenticated if that wiring is ever broken by a
// future change (e.g. a second transport added to mcpServer).
var errNoTokenInfo = errors.New("no verified identity on context")

// identityMiddleware reads the caller's user ID that middleware.RequireAuth
// already wrote onto context at the HTTP layer (see New) and writes it into
// logpkg the same way REST's handlers see it, so tool handlers only depend
// on those shared, transport-agnostic packages. Nothing populates ctxpkg's
// groups value on this path (mykeebs-api's tokens carry no groups-equivalent
// claim), so groups handling was dropped rather than kept as dead code
// that always sees nil.
func identityMiddleware() sdkmcp.Middleware {
	return func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
		return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
			userID, ok := ctxpkg.UserID(ctx)
			if !ok {
				logpkg.FromContext(ctx).Error("MCP request reached tool dispatch with no verified identity on context")
				return nil, errNoTokenInfo
			}

			ctx = logpkg.WithLogger(ctx, logpkg.WithUserID(logpkg.FromContext(ctx), userID))

			return next(ctx, method, req)
		}
	}
}

// ResourceURL is the MCP endpoint's URL as reached by r.
func ResourceURL(r *http.Request) string {
	return schemeOf(r) + "://" + r.Host + "/mcp"
}

func schemeOf(r *http.Request) string {
	if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
		return fp
	}
	return "https"
}

// metadataHandler serves RFC 9728 Protected Resource Metadata, building the
// "resource" field from the request's Host header (see Handlers doc comment
// for why this is derived per-request rather than statically configured).
func metadataHandler(issuerURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		metadata := oauthex.ProtectedResourceMetadata{
			Resource:             ResourceURL(r),
			AuthorizationServers: []string{issuerURL},
			ScopesSupported:      []string{"openid", "email", "profile", "offline_access"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(metadata)
	})
}
