package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/cascadedelete"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

var errKeycapSetNotFound = errors.New("keycap set not found")

var errKeycapSetAlreadyExists = errors.New("keycap set already exists")

var listKeycapSetsTool = &mcp.Tool{
	Name:        "list_keycap_sets",
	Description: "Lists keycap sets in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_keycap_set for a single set's full details, including its kits. Omit user_id to list your own keycap sets.",
}

var getKeycapSetTool = &mcp.Tool{
	Name:        "get_keycap_set",
	Description: "Returns the full details of one keycap set, including its kits. Each kit reports has_image rather than the image itself. Omit user_id to read from your own collection.",
}

var createKeycapSetTool = &mcp.Tool{
	Name:        "create_keycap_set",
	Description: "Adds a keycap set to your own collection. profile and material must be approved lookup values - call list_lookups and get_lookup to see them. Kits aren't set here - add them afterward with create_keycap_kit.",
}

var updateKeycapSetTool = &mcp.Tool{
	Name:        "update_keycap_set",
	Description: "Replaces a keycap set in your own collection. Every field is replaced, so omitting an optional field clears it; send the full keycap set, not just the fields you want to change. Kits are preserved untouched - manage them with the kit tools instead.",
}

var deleteKeycapSetTool = &mcp.Tool{
	Name:        "delete_keycap_set",
	Description: "Removes a keycap set from your own collection, along with any kit images. Idempotent: deleting a keycap set that isn't there succeeds.",
}

var createKeycapKitTool = &mcp.Tool{
	Name:        "create_keycap_kit",
	Description: "Adds a kit to a keycap set in your own collection. purchase.vendor and purchase.order_status must be approved lookup values - call list_lookups and get_lookup to see them. To set the kit's image, call set_keycap_kit_image afterward.",
}

var updateKeycapKitTool = &mcp.Tool{
	Name:        "update_keycap_kit",
	Description: "Replaces a kit on a keycap set in your own collection. Every field is replaced, so omitting an optional field clears it; send the full kit, not just the fields you want to change. The kit's image, if any, is preserved - manage it with the image tools instead.",
}

var deleteKeycapKitTool = &mcp.Tool{
	Name:        "delete_keycap_kit",
	Description: "Removes a kit from a keycap set in your own collection, along with its image if it has one. Idempotent: deleting a kit that isn't there succeeds.",
}

var setKeycapKitImageTool = &mcp.Tool{
	Name:        "set_keycap_kit_image",
	Description: "Mints a presigned URL to upload a kit's image to. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new image starts pending until it's checked, and only you can see it until it's approved. A rejected image is shown only to you, with the reason, until you replace or delete it.",
}

var deleteKeycapKitImageTool = &mcp.Tool{
	Name:        "delete_keycap_kit_image",
	Description: "Removes a kit's image. Idempotent: deleting a kit's image when it doesn't have one succeeds.",
}

func handleListKeycapSets(
	repo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.ListKeycapSetsInput, schema.ListKeycapSetsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeycapSetsInput) (*mcp.CallToolResult, schema.ListKeycapSetsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListKeycapSetsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		sets, nextCursor, err := repo.List(ctx, ownerID, visibilities, listLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListKeycapSetsOutput{}, errors.New("invalid cursor; restart from the first page")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing keycap sets", log.Error, err)
			return nil, schema.ListKeycapSetsOutput{}, errors.New("failed to list keycap sets")
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err)
			return nil, schema.ListKeycapSetsOutput{}, errors.New("failed to list keycap sets")
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		items := make([]schema.KeycapSetSummary, len(sets))
		for i, ks := range sets {
			items[i] = repomcp.KeycapSet{}.ToMCPSummary(ks, isOwner, ownerPrefs)
		}

		return nil, schema.ListKeycapSetsOutput{KeycapSets: items, NextCursor: nextCursor}, nil
	}
}

func handleGetKeycapSet(
	repo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.GetKeycapSetInput, schema.GetKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeycapSetInput) (*mcp.CallToolResult, schema.GetKeycapSetOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetKeycapSetOutput{}, err
		}

		ks, err := ownedReadable(ctx, repo.Get, func(ks repository.KeycapSet) repository.Visibility { return ks.Visibility },
			"keycap set", errKeycapSetNotFound, log.KeycapSetID, in.UserID, in.KeycapSetID)
		if err != nil {
			return nil, schema.GetKeycapSetOutput{}, err
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeycapSetID, in.KeycapSetID)
			return nil, schema.GetKeycapSetOutput{}, errors.New("failed to get keycap set")
		}

		return nil, schema.GetKeycapSetOutput{KeycapSet: repomcp.KeycapSet{}.ToMCP(*ks, isOwner, ownerPrefs)}, nil
	}
}

func handleCreateKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateKeycapSetInput, schema.CreateKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeycapSetInput) (*mcp.CallToolResult, schema.CreateKeycapSetOutput, error) {
		ks, err := validatedKeycapSet(ctx, in.KeycapSetInput)
		if err != nil {
			return nil, schema.CreateKeycapSetOutput{}, err
		}

		ks.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeycapSetID, ks.ID)
			return nil, schema.CreateKeycapSetOutput{}, errors.New("failed to create keycap set")
		}

		created, err := keycapSetRepo.Create(ctx, ks)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateKeycapSetOutput{}, errKeycapSetAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating keycap set", log.KeycapSetID, ks.ID, log.Error, err)
			return nil, schema.CreateKeycapSetOutput{}, errors.New("failed to create keycap set")
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.CreateKeycapSetOutput{KeycapSet: repomcp.KeycapSet{}.ToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateKeycapSetInput, schema.UpdateKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeycapSetInput) (*mcp.CallToolResult, schema.UpdateKeycapSetOutput, error) {
		ks, err := validatedKeycapSet(ctx, in.KeycapSetInput)
		if err != nil {
			return nil, schema.UpdateKeycapSetOutput{}, err
		}

		ks.ID = in.KeycapSetID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeycapSetID, ks.ID)
			return nil, schema.UpdateKeycapSetOutput{}, errors.New("failed to update keycap set")
		}

		updated, err := keycapSetRepo.Update(ctx, ks)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, ks.ID); mutErr != nil {
			return nil, schema.UpdateKeycapSetOutput{}, mutErr
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.UpdateKeycapSetOutput{KeycapSet: repomcp.KeycapSet{}.ToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapSetInput, schema.DeleteKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapSetInput) (*mcp.CallToolResult, schema.DeleteKeycapSetOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeycapSetOutput{}, err
		}

		result, err := cascadedelete.DeleteKeycapSet(ctx, keycapSetRepo, buildRepo, buildImages, images, ownerID, in.KeycapSetID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteKeycapSetOutput{}, fmt.Errorf("keycap set is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(ctx).Error("deleting keycap set", log.KeycapSetID, in.KeycapSetID, log.Error, err)
			return nil, schema.DeleteKeycapSetOutput{}, errors.New("failed to delete keycap set")
		}

		return nil, schema.DeleteKeycapSetOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func validatedKeycapSet(
	ctx context.Context,
	in schema.KeycapSetInput,
) (repository.KeycapSet, error) {
	ks := repomcp.KeycapSet{}.FromMCP(in)

	if err := lookupErrors(lookup.ValidateKeycapSet(ctx, ks)); err != nil {
		return repository.KeycapSet{}, err
	}

	return ks, nil
}

func handleCreateKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateKeycapKitInput, schema.CreateKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeycapKitInput) (*mcp.CallToolResult, schema.CreateKeycapKitOutput, error) {
		kit, err := validatedKeycapKit(ctx, in.KeycapKitInput)
		if err != nil {
			return nil, schema.CreateKeycapKitOutput{}, err
		}

		kit.KitID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, kit.KitID)
			return nil, schema.CreateKeycapKitOutput{}, errors.New("failed to create keycap kit")
		}

		created, err := keycapSetRepo.AddKit(ctx, in.KeycapSetID, kit, in.Primary)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, kit.KitID); mutErr != nil {
			return nil, schema.CreateKeycapKitOutput{}, mutErr
		}

		// isOwner: true - a kit is always added to the caller's own set.
		return nil, schema.CreateKeycapKitOutput{KeycapKit: repomcp.KeycapSet{}.KitToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateKeycapKitInput, schema.UpdateKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeycapKitInput) (*mcp.CallToolResult, schema.UpdateKeycapKitOutput, error) {
		kit, err := validatedKeycapKit(ctx, in.KeycapKitInput)
		if err != nil {
			return nil, schema.UpdateKeycapKitOutput{}, err
		}

		kit.KitID = in.KitID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, kit.KitID)
			return nil, schema.UpdateKeycapKitOutput{}, errors.New("failed to update keycap kit")
		}

		updated, err := keycapSetRepo.UpdateKit(ctx, in.KeycapSetID, kit, in.Primary)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.UpdateKeycapKitOutput{}, mutErr
		}

		// isOwner: true - a kit is always updated on the caller's own set.
		return nil, schema.UpdateKeycapKitOutput{KeycapKit: repomcp.KeycapSet{}.KitToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapKitInput, schema.DeleteKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapKitInput) (*mcp.CallToolResult, schema.DeleteKeycapKitOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeycapKitOutput{}, err
		}

		result, err := cascadedelete.DeleteKeycapKit(ctx, keycapSetRepo, buildRepo, buildImages, images, ownerID, in.KeycapSetID, in.KitID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteKeycapKitOutput{}, fmt.Errorf("keycap kit is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.DeleteKeycapKitOutput{}, mutErr
		}

		return nil, schema.DeleteKeycapKitOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func handleSetKeycapKitImage(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.SetKeycapKitImageInput, schema.SetKeycapKitImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.SetKeycapKitImageInput) (*mcp.CallToolResult, schema.SetKeycapKitImageOutput, error) {
		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.SetKeycapKitImageOutput{}, err
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.SetKeycapKitImageOutput{}, err
		}

		ks, err := keycapSetRepo.Get(ctx, ownerID, in.KeycapSetID)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.SetKeycapKitImageOutput{}, mutErr
		}

		kit, ok := ks.Kits[in.KitID]
		if !ok {
			return nil, schema.SetKeycapKitImageOutput{}, errMutationNotFound
		}

		if err := reserveUpload(ctx, uploads, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); err != nil {
			return nil, schema.SetKeycapKitImageOutput{}, err
		}

		key, err := repository.NewKeycapKitImageKey(ctx, in.KeycapSetID, in.KitID, uuid.NewString())
		if err != nil {
			log.FromContext(ctx).Error("building keycap kit image key", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
		}

		uploadURL, err := images.PresignPut(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning keycap kit image upload", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
		}

		if kit.ImagePath != nil {
			if err := images.Delete(ctx, *kit.ImagePath); err != nil {
				log.FromContext(ctx).Error("deleting previous keycap kit image object", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
				return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
			}
		}

		err = keycapSetRepo.SetKitImagePath(ctx, in.KeycapSetID, in.KitID, key)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.SetKeycapKitImageOutput{}, mutErr
		}

		return nil, schema.SetKeycapKitImageOutput{UploadURL: uploadURL}, nil
	}
}

func handleDeleteKeycapKitImage(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapKitImageInput, schema.DeleteKeycapKitImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapKitImageInput) (*mcp.CallToolResult, schema.DeleteKeycapKitImageOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeycapKitImageOutput{}, err
		}

		ks, err := keycapSetRepo.Get(ctx, ownerID, in.KeycapSetID)
		if mutErr := handleMutationError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.DeleteKeycapKitImageOutput{}, mutErr
		}

		kit, ok := ks.Kits[in.KitID]
		if !ok {
			return nil, schema.DeleteKeycapKitImageOutput{}, errMutationNotFound
		}

		if kit.ImagePath == nil {
			return nil, schema.DeleteKeycapKitImageOutput{}, nil
		}

		if err := images.Delete(ctx, *kit.ImagePath); err != nil {
			log.FromContext(ctx).Error("deleting keycap kit image object", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.DeleteKeycapKitImageOutput{}, errors.New("failed to delete kit image")
		}

		_, err = keycapSetRepo.ClearKitImagePath(ctx, in.KeycapSetID, in.KitID)
		if mutErr := handleClearImageError(ctx, err, log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID); mutErr != nil {
			return nil, schema.DeleteKeycapKitImageOutput{}, mutErr
		}

		return nil, schema.DeleteKeycapKitImageOutput{}, nil
	}
}

func validatedKeycapKit(
	ctx context.Context,
	in schema.KeycapKitInput,
) (repository.KeycapKit, error) {
	kit := repomcp.KeycapSet{}.KitFromMCP(in)

	if err := lookupErrors(lookup.ValidateKeycapKit(ctx, kit)); err != nil {
		return repository.KeycapKit{}, err
	}

	return kit, nil
}
