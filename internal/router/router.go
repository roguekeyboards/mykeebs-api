package router

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/auth"
	"github.com/roguekeyboards/mykeebs-api/internal/consent"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp"
	"github.com/roguekeyboards/mykeebs-api/internal/middleware"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// New builds the application's http.Handler. verifier authenticates every
// REST request and /mcp. Additional entities/routes are added here in
// later issues, on this same handler.
//
// issuerURL configures the MCP endpoint's RFC 9728 Protected Resource
// Metadata (the OIDC issuer MCP clients should authenticate against); the
// metadata's "resource" field is derived per-request rather than passed in
// statically — see [github.com/roguekeyboards/mykeebs-api/internal/mcp.Handlers]
// for why. idpConsentPublicToken configures the GET /authorize consent
// page (see internal/consent). version is advertised to MCP clients in the
// server's initialize handshake.
func New(
	verifier *auth.Verifier,
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
	ownerImageStore repository.OwnerImageStore,
	deleteNotices accountdelete.Notifier,
	uploads *uploadlimit.Limiter,
	issuerURL, idpConsentPublicToken, version string,
) http.Handler {
	validate := restOpenAPIValidator()

	sr := repoapi.Switch{Images: switchImageStore}
	er := repoapi.ECPart{Images: ecPartImageStore}
	kr := repoapi.Keyboard{Images: keyboardImageStore}
	kcr := repoapi.KeycapSet{Images: imageStore}
	br := repoapi.Build{
		Images:         buildImageStore,
		KitImages:      imageStore,
		KeyboardImages: keyboardImageStore,
		SwitchImages:   switchImageStore,
		ECPartImages:   ecPartImageStore,
		KeyboardRepo:   keyboardRepo,
		SwitchRepo:     switchRepo,
		ECPartRepo:     ecPartRepo,
		KeycapSetRepo:  keycapSetRepo,
	}
	pr := repoapi.Profile{Images: profileImageStore}

	account := accountdelete.Account{
		Images:     ownerImageStore,
		Builds:     buildRepo,
		Keyboards:  keyboardRepo,
		Switches:   switchRepo,
		ECParts:    ecPartRepo,
		KeycapSets: keycapSetRepo,
		Profiles:   profileRepo,
		Notices:    deleteNotices,
	}

	ownerPrefs := middleware.OwnerPreferences(profileRepo)

	mux := http.NewServeMux()

	// Not part of api/openapi.yaml, so not wrapped in validate - see
	// internal/consent.
	mux.Handle("GET /authorize", consent.Handler(idpConsentPublicToken, issuerURL))

	// These pages moved to mykeebs.com; old links still reach them.
	for _, path := range []string{"/privacy", "/terms", "/help/mcp"} {
		mux.Handle("GET "+path, http.RedirectHandler("https://mykeebs.com"+path, http.StatusMovedPermanently))
	}

	// security: [] in api/openapi.yaml - always anonymous. No PUT/DELETE:
	// lookup categories are static, deploy-time data (internal/lookup),
	// not writable at runtime.
	mux.Handle("GET /v1/lookups", validate(http.HandlerFunc(handlers.ListLookups)))
	mux.Handle("GET /v1/lookups/{category}", validate(http.HandlerFunc(handlers.GetLookup)))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers see
	// only public switches (see [github.com/roguekeyboards/mykeebs-api/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/switches",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.ListSwitches(switchRepo, sr)))))
	mux.Handle("GET /v1/users/{userId}/switches/{switchId}",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.GetSwitch(switchRepo, sr)))))
	mux.Handle("POST /v1/users/{userId}/switches",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateSwitch(switchRepo, sr)))))
	mux.Handle("PUT /v1/users/{userId}/switches/{switchId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateSwitch(switchRepo, sr)))))
	mux.Handle("DELETE /v1/users/{userId}/switches/{switchId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteSwitch(switchRepo, buildRepo, buildImageStore, switchImageStore))))
	mux.Handle("POST /v1/users/{userId}/switches/{switchId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetSwitchImage(switchRepo, switchImageStore, uploads))))
	mux.Handle("DELETE /v1/users/{userId}/switches/{switchId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteSwitchImage(switchRepo, switchImageStore))))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers see
	// only public EC parts.
	mux.Handle("GET /v1/users/{userId}/ec-parts",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.ListECParts(ecPartRepo, er)))))
	mux.Handle("GET /v1/users/{userId}/ec-parts/{ecPartId}",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.GetECPart(ecPartRepo, er)))))
	mux.Handle("POST /v1/users/{userId}/ec-parts",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateECPart(ecPartRepo, er)))))
	mux.Handle("PUT /v1/users/{userId}/ec-parts/{ecPartId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateECPart(ecPartRepo, er)))))
	mux.Handle("DELETE /v1/users/{userId}/ec-parts/{ecPartId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteECPart(ecPartRepo, buildRepo, buildImageStore, ecPartImageStore))))
	mux.Handle("POST /v1/users/{userId}/ec-parts/{ecPartId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetECPartImage(ecPartRepo, ecPartImageStore, uploads))))
	mux.Handle("DELETE /v1/users/{userId}/ec-parts/{ecPartId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteECPartImage(ecPartRepo, ecPartImageStore))))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers see
	// only public keyboards (see [github.com/roguekeyboards/mykeebs-api/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/keyboards",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.ListKeyboards(keyboardRepo, kr)))))
	mux.Handle("GET /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.GetKeyboard(keyboardRepo, kr)))))
	mux.Handle("POST /v1/users/{userId}/keyboards",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateKeyboard(keyboardRepo, kr)))))
	mux.Handle("PUT /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateKeyboard(keyboardRepo, kr)))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboard(keyboardRepo, buildRepo, buildImageStore, keyboardImageStore))))
	mux.Handle("POST /v1/users/{userId}/keyboards/{keyboardId}/plates",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateKeyboardPlate(keyboardRepo, kr)))))
	mux.Handle("PUT /v1/users/{userId}/keyboards/{keyboardId}/plates/{plateId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateKeyboardPlate(keyboardRepo, kr)))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}/plates/{plateId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboardPlate(keyboardRepo, buildRepo, buildImageStore))))
	mux.Handle("POST /v1/users/{userId}/keyboards/{keyboardId}/pcbs",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateKeyboardPCB(keyboardRepo, kr)))))
	mux.Handle("PUT /v1/users/{userId}/keyboards/{keyboardId}/pcbs/{pcbId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateKeyboardPCB(keyboardRepo, kr)))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}/pcbs/{pcbId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboardPCB(keyboardRepo, buildRepo, buildImageStore))))
	mux.Handle("POST /v1/users/{userId}/keyboards/{keyboardId}/images",
		middleware.RequireAuthorizerIdentity(validate(handlers.AddKeyboardImage(keyboardRepo, keyboardImageStore, uploads))))
	mux.Handle("DELETE /v1/users/{userId}/keyboards/{keyboardId}/images/{imageId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeyboardImage(keyboardRepo, keyboardImageStore))))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers see
	// only public keycap sets (see [github.com/roguekeyboards/mykeebs-api/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/keycap-sets",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.ListKeycapSets(keycapSetRepo, kcr)))))
	mux.Handle("GET /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.GetKeycapSet(keycapSetRepo, kcr)))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateKeycapSet(keycapSetRepo, kcr)))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateKeycapSet(keycapSetRepo, kcr)))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapSet(keycapSetRepo, buildRepo, buildImageStore, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateKeycapKit(keycapSetRepo, kcr)))))
	mux.Handle("PUT /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateKeycapKit(keycapSetRepo, kcr)))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapKit(keycapSetRepo, buildRepo, buildImageStore, imageStore))))
	mux.Handle("POST /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetKeycapKitImage(keycapSetRepo, imageStore, uploads))))
	mux.Handle("DELETE /v1/users/{userId}/keycap-sets/{keycapSetId}/kits/{kitId}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteKeycapKitImage(keycapSetRepo, imageStore))))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers see
	// only public builds (see [github.com/roguekeyboards/mykeebs-api/internal/authz.ReadableVisibilities]).
	mux.Handle("GET /v1/users/{userId}/builds",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.ListBuilds(buildRepo, br)))))
	mux.Handle("POST /v1/users/{userId}/builds",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.CreateBuild(buildRepo, br, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo)))))
	mux.Handle("GET /v1/users/{userId}/builds/{buildId}",
		middleware.OptionalAuth(verifier)(ownerPrefs(validate(handlers.GetBuild(buildRepo, br)))))
	mux.Handle("PUT /v1/users/{userId}/builds/{buildId}",
		middleware.RequireAuthorizerIdentity(ownerPrefs(validate(handlers.UpdateBuild(buildRepo, br, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo)))))
	mux.Handle("DELETE /v1/users/{userId}/builds/{buildId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteBuild(buildRepo, buildImageStore))))
	mux.Handle("POST /v1/users/{userId}/builds/{buildId}/images",
		middleware.RequireAuthorizerIdentity(validate(handlers.AddBuildImage(buildRepo, buildImageStore, uploads))))
	mux.Handle("DELETE /v1/users/{userId}/builds/{buildId}/images/{imageId}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteBuildImage(buildRepo, buildImageStore))))

	// security: [{}, BearerAuth] in api/openapi.yaml - anonymous callers may
	// read discoverable profiles; the handler applies the
	// discoverable-or-owner rule itself (see internal/profileread).
	mux.Handle("GET /v1/profile/{identifier}",
		middleware.OptionalAuth(verifier)(validate(handlers.GetProfile(profileRepo, pr))))
	// Same path as the GET, POST only. Default OidcAuthorizer at the gateway
	// (security: [BearerAuth] in api/openapi.yaml, no anonymous override);
	// the handler requires {identifier} to be the caller's own subject via
	// authz.IsOwner (a username there, or anyone else's subject, is 404).
	mux.Handle("POST /v1/profile/{identifier}",
		middleware.RequireAuthorizerIdentity(validate(handlers.CreateProfile(profileRepo, pr))))
	// Same path/auth as the POST - PUT only. {identifier} must be the
	// caller's own subject (authz.IsOwner); a full replace of the
	// body-settable fields, avatar untouched.
	mux.Handle("PUT /v1/profile/{identifier}",
		middleware.RequireAuthorizerIdentity(validate(handlers.UpdateProfile(profileRepo, pr))))
	// Same path/auth as the POST/PUT - DELETE only. {identifier} must be the
	// caller's own subject (authz.IsOwner). Deletes the whole account, every
	// item included; idempotent: nothing left is still 204.
	mux.Handle("DELETE /v1/profile/{identifier}",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteProfile(account))))
	// The public directory: anonymous OK, only discoverable profiles returned.
	mux.Handle("GET /v1/profiles",
		middleware.OptionalAuth(verifier)(validate(handlers.ListProfiles(profileRepo, pr))))
	// Avatar upload/removal - single-slot image, the switch pattern. Default
	// OidcAuthorizer at the gateway; {identifier} must be the caller's own
	// subject (authz.IsOwner), and the response to POST is a presigned S3
	// PUT URL, not the image bytes.
	mux.Handle("POST /v1/profile/{identifier}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.SetProfileImage(profileRepo, profileImageStore, uploads))))
	mux.Handle("DELETE /v1/profile/{identifier}/image",
		middleware.RequireAuthorizerIdentity(validate(handlers.DeleteProfileImage(profileRepo, profileImageStore))))

	// MCP: /mcp verifies auth in-process (middleware.RequireAuth), not via
	// API Gateway's native authorizer - see that middleware's doc comment
	// for why (a spec-compliant 401 needs a WWW-Authenticate header naming
	// the RFC 9728 metadata URL, which the gateway's own fixed rejection
	// response can't carry). internal/gatewayroutes gives /mcp no gateway
	// authorizer accordingly. Not wrapped in validate: api/openapi.yaml only covers
	// the REST surface.
	mcpHandlers := mcp.New(switchRepo, switchImageStore, ecPartRepo, ecPartImageStore, keyboardRepo, keyboardImageStore, keycapSetRepo, imageStore, buildRepo, buildImageStore, profileRepo, profileImageStore, account, uploads, verifier, issuerURL, version)
	mux.Handle("/mcp", mcpHandlers.Streamable)
	mux.Handle(mcpHandlers.MetadataPath, mcpHandlers.Metadata)
	mux.Handle(mcpHandlers.RootMetadataPath, mcpHandlers.Metadata)

	return middleware.Logging(middleware.Recover(mux))
}

// restOpenAPIValidator validates incoming REST requests' shape against
// api/openapi.yaml. Auth stays enforced by
// [github.com/roguekeyboards/mykeebs-api/internal/middleware], not here (see
// [openapi3filter.NoopAuthenticationFunc]); open-vocabulary lookup values
// stay hand-validated per handler (e.g.
// [github.com/roguekeyboards/mykeebs-api/internal/lookup.ValidateSwitch]),
// since that isn't expressible as a static schema.
func restOpenAPIValidator() func(http.Handler) http.Handler {
	spec, err := api.GetSpec()
	if err != nil {
		log.Fatalf("router: loading embedded OpenAPI spec: %v", err)
	}

	return nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		// Deployed Host varies per stack; spec's servers URL is fixed.
		DoNotValidateServers: true,
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			// Report every schema violation in a request, not just the
			// first, so validationErrorHandler can populate a complete
			// invalid_params list in one response.
			MultiError: true,
		},
		ErrorHandlerWithOpts: validationErrorHandler,
	})
}

// validationErrorHandler translates nethttp-middleware's OpenAPI validation
// failures into the same RFC 9457 application/problem+json shape used
// everywhere else in the API (see
// [github.com/roguekeyboards/mykeebs-api/internal/problem]), rather than the
// middleware's default plain-text body. Requests to paths/methods absent
// from api/openapi.yaml entirely (opts.MatchedRoute == nil) are reported as
// a plain 404, since there is no schema to name invalid_params against.
func validationErrorHandler(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts nethttpmiddleware.ErrorHandlerOpts) {
	if opts.MatchedRoute == nil {
		problem.NotFound(w, "resource not found")
		return
	}

	invalidParams := collectInvalidParams(err)
	if len(invalidParams) == 0 {
		problem.BadRequest(w, err.Error())
		return
	}

	problem.ValidationFailed(w, "request failed schema validation", invalidParams)
}

// collectInvalidParams walks err - an openapi3.MultiError of
// *openapi3filter.RequestError, each possibly itself wrapping an
// openapi3.MultiError of *openapi3.SchemaError (one per violated field) -
// into one problem.InvalidParam per violation. Returns nil if err doesn't
// match this shape (e.g. a malformed-JSON body, which has no field path).
func collectInvalidParams(err error) []problem.InvalidParam {
	var params []problem.InvalidParam

	multi, ok := errors.AsType[openapi3.MultiError](err)
	if !ok {
		multi = openapi3.MultiError{err}
	}

	for _, e := range multi {
		reqErr, ok := errors.AsType[*openapi3filter.RequestError](e)
		if !ok || reqErr.Err == nil {
			continue
		}

		// A parameter-level failure (query/path/header/cookie) names itself
		// via Parameter.Name - reqErr.Err here is often a plain parse/
		// required error, not a *openapi3.SchemaError with a JSONPointer,
		// so schemaErrorsFrom below (body-only) can't name it and would
		// otherwise mislabel it "body".
		if reqErr.Parameter != nil {
			params = append(params, problem.InvalidParam{Name: reqErr.Parameter.Name, Reason: reqErr.Err.Error()})
			continue
		}

		params = append(params, schemaErrorsFrom(reqErr.Err)...)
	}

	return params
}

// schemaErrorsFrom flattens err - a *openapi3.SchemaError or an
// openapi3.MultiError of them - into invalid_params entries, naming each by
// its JSON pointer path (e.g. "material.stem").
func schemaErrorsFrom(err error) []problem.InvalidParam {
	if schemaErr, ok := errors.AsType[*openapi3.SchemaError](err); ok {
		name := strings.Join(schemaErr.JSONPointer(), ".")
		if name == "" {
			name = "body"
		}

		return []problem.InvalidParam{{Name: name, Reason: schemaErr.Reason}}
	}

	if multi, ok := errors.AsType[openapi3.MultiError](err); ok {
		var params []problem.InvalidParam
		for _, e := range multi {
			params = append(params, schemaErrorsFrom(e)...)
		}

		return params
	}

	return nil
}
