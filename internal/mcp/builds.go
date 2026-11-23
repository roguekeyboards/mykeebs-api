package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/buildrefs"
	apictx "github.com/roguekeyboards/mykeebs-api/internal/ctx"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

var errBuildAlreadyExists = errors.New("build already exists")

var errBuildNotFound = errors.New("build not found")

// errNoCaller mirrors [errNoTokenInfo]'s fail-closed shape: unreachable in
// practice since [identityMiddleware] always sets a caller ID before a tool
// handler runs, but this fails closed rather than validating references
// against an empty ownerID if that wiring is ever broken.
var errNoCaller = errors.New("no caller identity on context")

var createBuildTool = &mcp.Tool{
	Name:        "create_build",
	Description: "Adds a build to your own collection, recording an actual keyboard you've assembled from a keyboard, switches or EC parts, keycap kit(s), and stabilizer/mount/foam details. build_type is an approved build_type lookup value (MX when omitted): an EC build lists ec_parts and no switches, any other lists switches and no ec_parts. keyboard must be the id of a Keyboard resource you own, switches[].switch must each be the id of a Switch resource you own, ec_parts[].ec_part must each be the id of an EC part you own, and keycap_kits[].keycap_set/kit must each name a KeycapSet you own and one of its kits, and plate and pcb, if set, must be the id of one of that keyboard's plates and pcbs (call get_keyboard to see them) - all are verified to exist and belong to you. stabs.name, stabs.mount_type, case_mount_type.type, and case_mount_type.durometer must be approved lookup values - call list_lookups and get_lookup to see them. Images aren't set here - a future tool adds them afterward.",
}

var getBuildTool = &mcp.Tool{
	Name:        "get_build",
	Description: "Returns the full details of one build. Images are reported via has_images rather than URLs - a future tool fetches them on demand. Omit user_id to read from your own collection.",
}

var listBuildsTool = &mcp.Tool{
	Name:        "list_builds",
	Description: "Lists builds in a user's collection, most useful for browsing. Returns an abbreviated shape, including the referenced keyboard's brand/name; call get_build for a single build's full details. Omit user_id to list your own builds. Set keyboard_id to only return builds built on that keyboard.",
}

var updateBuildTool = &mcp.Tool{
	Name:        "update_build",
	Description: "Replaces every field of one of your own builds - fields omitted from the call are cleared, not left unchanged; send build_type again for an EC build, since an omitted build_type means MX. An EC build lists ec_parts and no switches, any other lists switches and no ec_parts. keyboard must be the id of a Keyboard resource you own, switches[].switch must each be the id of a Switch resource you own, ec_parts[].ec_part must each be the id of an EC part you own, and keycap_kits[].keycap_set/kit must each name a KeycapSet you own and one of its kits, and plate and pcb, if set, must be the id of one of that keyboard's plates and pcbs (call get_keyboard to see them) - all are verified to exist and belong to you. stabs.name, stabs.mount_type, case_mount_type.type, and case_mount_type.durometer must be approved lookup values - call list_lookups and get_lookup to see them. Images are managed separately and are unaffected by this call.",
}

var deleteBuildTool = &mcp.Tool{
	Name:        "delete_build",
	Description: "Removes a build from your own collection, along with any images. Idempotent: deleting a build that isn't there succeeds.",
}

var addBuildImageTool = &mcp.Tool{
	Name:        "add_build_image",
	Description: "Mints a presigned URL to upload a new image to one of your own builds. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new image starts pending until it's checked, and only you can see it until it's approved. A rejected image is shown only to you, with the reason, and still counts toward the image limit until you delete it. A build may have at most 10 images; this always adds a new one rather than replacing an existing image.",
}

var deleteBuildImageTool = &mcp.Tool{
	Name:        "delete_build_image",
	Description: "Removes one image from one of your own builds, along with the underlying image object. Idempotent: deleting an image that isn't there succeeds.",
}

var listBuildImagesTool = &mcp.Tool{
	Name:        "list_build_images",
	Description: "Lists the ids of a build's images. get_build/list_builds only report whether any exist via has_images/has_image - call this to get their ids, e.g. before deleting one. Its owner gets every image with its moderation status; anyone else gets approved images only.",
}

func handleListBuilds(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.ListBuildsInput, schema.ListBuildsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListBuildsInput) (*mcp.CallToolResult, schema.ListBuildsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListBuildsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		builds, nextCursor, err := buildRepo.List(ctx, ownerID, visibilities, in.KeyboardID, listLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListBuildsOutput{}, errors.New("invalid cursor; restart from the first page")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing builds", log.Error, err)
			return nil, schema.ListBuildsOutput{}, errors.New("failed to list builds")
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err)
			return nil, schema.ListBuildsOutput{}, errors.New("failed to list builds")
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		mapper := repomcp.Build{
			KeyboardRepo:  keyboardRepo,
			SwitchRepo:    switchRepo,
			ECPartRepo:    ecPartRepo,
			KeycapSetRepo: keycapSetRepo,
		}

		items := make([]schema.BuildSummary, len(builds))
		errs := make([]error, len(builds))

		var wg sync.WaitGroup
		for i, b := range builds {
			wg.Add(1)
			go func(i int, b repository.Build) {
				defer wg.Done()

				summary, err := mapper.ToMCPSummary(ctx, b, isOwner, ownerPrefs)
				if err != nil {
					errs[i] = fmt.Errorf("mapping build %q to MCP summary: %w", b.ID, err)
					return
				}
				items[i] = summary
			}(i, b)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(ctx).Error("mapping builds to MCP summaries", log.Error, err)
			return nil, schema.ListBuildsOutput{}, errors.New("failed to list builds")
		}

		return nil, schema.ListBuildsOutput{Builds: items, NextCursor: nextCursor}, nil
	}
}

func handleGetBuild(
	repo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.GetBuildInput, schema.GetBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetBuildInput) (*mcp.CallToolResult, schema.GetBuildOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetBuildOutput{}, err
		}

		b, err := ownedReadable(ctx, repo.Get, func(b repository.Build) repository.Visibility { return b.Visibility },
			"build", errBuildNotFound, log.BuildID, in.UserID, in.BuildID)
		if err != nil {
			return nil, schema.GetBuildOutput{}, err
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.BuildID, in.BuildID)
			return nil, schema.GetBuildOutput{}, errors.New("failed to get build")
		}

		out, err := repomcp.Build{KeyboardRepo: keyboardRepo}.ToMCPResolved(ctx, *b, isOwner, ownerPrefs)
		if err != nil {
			log.FromContext(ctx).Error("mapping build to MCP", log.Error, err, log.BuildID, in.BuildID)
			return nil, schema.GetBuildOutput{}, errors.New("failed to get build")
		}

		return nil, schema.GetBuildOutput{Build: out}, nil
	}
}

func handleCreateBuild(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateBuildInput, schema.CreateBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateBuildInput) (*mcp.CallToolResult, schema.CreateBuildOutput, error) {
		b, err := validatedBuild(ctx, in.BuildInput)
		if err != nil {
			return nil, schema.CreateBuildOutput{}, err
		}

		ownerID, ok := apictx.UserID(ctx)
		if !ok {
			return nil, schema.CreateBuildOutput{}, errNoCaller
		}

		fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo)
		if err != nil {
			log.FromContext(ctx).Error("validating build references", log.Error, err)
			return nil, schema.CreateBuildOutput{}, errors.New("failed to validate build")
		}
		if len(fieldErrs) > 0 {
			return nil, schema.CreateBuildOutput{}, buildRefErrors(fieldErrs)
		}

		b.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.BuildID, b.ID)
			return nil, schema.CreateBuildOutput{}, errors.New("failed to create build")
		}

		created, err := buildRepo.Create(ctx, b)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateBuildOutput{}, errBuildAlreadyExists
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			return nil, schema.CreateBuildOutput{}, handleTransactionError(ctx, err, log.BuildID, b.ID)
		}
		if err != nil {
			log.FromContext(ctx).Error("creating build", log.BuildID, b.ID, log.Error, err)
			return nil, schema.CreateBuildOutput{}, errors.New("failed to create build")
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.CreateBuildOutput{Build: repomcp.Build{}.ToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateBuild(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateBuildInput, schema.UpdateBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateBuildInput) (*mcp.CallToolResult, schema.UpdateBuildOutput, error) {
		b, err := validatedBuild(ctx, in.BuildInput)
		if err != nil {
			return nil, schema.UpdateBuildOutput{}, err
		}

		ownerID, ok := apictx.UserID(ctx)
		if !ok {
			return nil, schema.UpdateBuildOutput{}, errNoCaller
		}

		fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo)
		if err != nil {
			log.FromContext(ctx).Error("validating build references", log.Error, err)
			return nil, schema.UpdateBuildOutput{}, errors.New("failed to validate build")
		}
		if len(fieldErrs) > 0 {
			return nil, schema.UpdateBuildOutput{}, buildRefErrors(fieldErrs)
		}

		b.ID = in.BuildID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.BuildID, b.ID)
			return nil, schema.UpdateBuildOutput{}, errors.New("failed to update build")
		}

		updated, err := buildRepo.Update(ctx, b)
		if mutErr := handleTransactionError(ctx, err, log.BuildID, b.ID); mutErr != nil {
			return nil, schema.UpdateBuildOutput{}, mutErr
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.UpdateBuildOutput{Build: repomcp.Build{}.ToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func buildRefErrors(fieldErrs []buildrefs.FieldError) error {
	reasons := make([]string, len(fieldErrs))
	for i, fe := range fieldErrs {
		if fe.Value == "" {
			reasons[i] = fe.Field + ": " + fe.Reason
			continue
		}
		reasons[i] = fmt.Sprintf("%s: %q %s", fe.Field, fe.Value, fe.Reason)
	}
	return errors.New(strings.Join(reasons, "; "))
}

func validatedBuild(ctx context.Context, in schema.BuildInput) (repository.Build, error) {
	b := repomcp.Build{}.FromMCP(in)

	if err := lookupErrors(lookup.ValidateBuild(ctx, b)); err != nil {
		return repository.Build{}, err
	}

	return b, nil
}

func handleDeleteBuild(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
) mcp.ToolHandlerFor[schema.DeleteBuildInput, schema.DeleteBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteBuildInput) (*mcp.CallToolResult, schema.DeleteBuildOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteBuildOutput{}, err
		}

		b, err := buildRepo.Get(ctx, ownerID, in.BuildID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.DeleteBuildOutput{}, nil
		}
		if err != nil {
			log.FromContext(ctx).Error("getting build", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.DeleteBuildOutput{}, errors.New("failed to delete build")
		}

		var errsMu sync.Mutex
		var errs []error
		var wg sync.WaitGroup
		for _, entry := range b.Images {
			wg.Add(1)
			go func(key repository.BuildImageKey) {
				defer wg.Done()
				if err := images.DeleteBuildImage(ctx, key); err != nil {
					errsMu.Lock()
					errs = append(errs, err)
					errsMu.Unlock()
				}
			}(entry.Path)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(ctx).Error("deleting build images", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.DeleteBuildOutput{}, errors.New("failed to delete build")
		}

		err = buildRepo.Delete(ctx, in.BuildID)
		if errors.Is(err, repository.ErrMutationConflict) {
			return nil, schema.DeleteBuildOutput{}, handleTransactionError(ctx, err, log.BuildID, in.BuildID)
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(ctx).Error("deleting build", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.DeleteBuildOutput{}, errors.New("failed to delete build")
		}

		return nil, schema.DeleteBuildOutput{}, nil
	}
}

func handleAddBuildImage(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.AddBuildImageInput, schema.AddBuildImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.AddBuildImageInput) (*mcp.CallToolResult, schema.AddBuildImageOutput, error) {
		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.AddBuildImageOutput{}, err
		}

		if err := reserveUpload(ctx, uploads, log.BuildID, in.BuildID); err != nil {
			return nil, schema.AddBuildImageOutput{}, err
		}

		imageID := uuid.NewString()

		key, err := repository.NewBuildImageKey(ctx, in.BuildID, imageID)
		if err != nil {
			log.FromContext(ctx).Error("building build image key", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.AddBuildImageOutput{}, errors.New("failed to add build image")
		}

		uploadURL, err := images.PresignPutBuildImage(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning build image upload", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.AddBuildImageOutput{}, errors.New("failed to add build image")
		}

		err = buildRepo.AddImage(ctx, in.BuildID, repository.BuildImage{ImageID: imageID, Path: key})
		if mutErr := handleMutationError(ctx, err, log.BuildID, in.BuildID); mutErr != nil {
			return nil, schema.AddBuildImageOutput{}, mutErr
		}

		return nil, schema.AddBuildImageOutput{ImageID: imageID, UploadURL: uploadURL}, nil
	}
}

func handleListBuildImages(
	repo repository.BuildRepository,
) mcp.ToolHandlerFor[schema.ListBuildImagesInput, schema.ListBuildImagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListBuildImagesInput) (*mcp.CallToolResult, schema.ListBuildImagesOutput, error) {
		b, err := ownedReadable(ctx, repo.Get, func(b repository.Build) repository.Visibility { return b.Visibility },
			"build", errBuildNotFound, log.BuildID, in.UserID, in.BuildID)
		if err != nil {
			return nil, schema.ListBuildImagesOutput{}, err
		}

		images := repomcp.Build{}.ImagesToMCP(*b, authz.IsOwner(ctx, b.UserID))

		return nil, schema.ListBuildImagesOutput{Images: images}, nil
	}
}

func handleDeleteBuildImage(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
) mcp.ToolHandlerFor[schema.DeleteBuildImageInput, schema.DeleteBuildImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteBuildImageInput) (*mcp.CallToolResult, schema.DeleteBuildImageOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteBuildImageOutput{}, err
		}

		b, err := buildRepo.Get(ctx, ownerID, in.BuildID)
		if mutErr := handleMutationError(ctx, err, log.BuildID, in.BuildID); mutErr != nil {
			return nil, schema.DeleteBuildImageOutput{}, mutErr
		}

		entry, ok := b.Images[in.ImageID]
		if !ok {
			return nil, schema.DeleteBuildImageOutput{}, nil
		}

		if err := images.DeleteBuildImage(ctx, entry.Path); err != nil {
			log.FromContext(ctx).Error("deleting build image object", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.DeleteBuildImageOutput{}, errors.New("failed to delete build image")
		}

		_, err = buildRepo.DeleteImage(ctx, in.BuildID, in.ImageID)
		if mutErr := handleClearImageError(ctx, err, log.BuildID, in.BuildID); mutErr != nil {
			return nil, schema.DeleteBuildImageOutput{}, mutErr
		}

		return nil, schema.DeleteBuildImageOutput{}, nil
	}
}
