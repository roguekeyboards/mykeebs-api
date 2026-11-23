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

var errSwitchNotFound = errors.New("switch not found")

var errSwitchAlreadyExists = errors.New("switch already exists")

var listSwitchesTool = &mcp.Tool{
	Name:        "list_switches",
	Description: "Lists switches in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_switch for a single switch's full details. Omit user_id to list your own switches.",
}

var getSwitchTool = &mcp.Tool{
	Name:        "get_switch",
	Description: "Returns the full details of one switch. Omit user_id to read from your own collection.",
}

var createSwitchTool = &mcp.Tool{
	Name:        "create_switch",
	Description: "Adds a switch to your own collection. type, material, spring.material, and purchase vendor/status must be approved lookup values - call list_lookups and get_lookup to see them.",
}

var updateSwitchTool = &mcp.Tool{
	Name:        "update_switch",
	Description: "Replaces a switch in your own collection. Every field is replaced, so omitting an optional field clears it; send the full switch, not just the fields you want to change.",
}

var deleteSwitchTool = &mcp.Tool{
	Name:        "delete_switch",
	Description: "Removes a switch from your own collection. Idempotent: deleting a switch that isn't there succeeds. on_delete controls what happens if a build still references this switch: \"block\" (default) fails and lists the blocking build ids; \"cascade\" deletes the switch and every referencing build.",
}

var setSwitchImageTool = &mcp.Tool{
	Name:        "set_switch_image",
	Description: "Mints a presigned URL to upload a switch's image to. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new image starts pending until it's checked, and only you can see it until it's approved. A rejected image is shown only to you, with the reason, until you replace or delete it.",
}

var deleteSwitchImageTool = &mcp.Tool{
	Name:        "delete_switch_image",
	Description: "Removes a switch's image. Idempotent: deleting a switch's image when it doesn't have one succeeds.",
}

func handleListSwitches(
	repo repository.SwitchRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.ListSwitchesInput, schema.ListSwitchesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListSwitchesInput) (*mcp.CallToolResult, schema.ListSwitchesOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListSwitchesOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		switches, nextCursor, err := repo.List(ctx, ownerID, visibilities, listLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListSwitchesOutput{}, errors.New("invalid cursor; restart from the first page")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing switches", log.Error, err)
			return nil, schema.ListSwitchesOutput{}, errors.New("failed to list switches")
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err)
			return nil, schema.ListSwitchesOutput{}, errors.New("failed to list switches")
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		items := make([]schema.SwitchSummary, len(switches))
		for i, sw := range switches {
			items[i] = repomcp.Switch{}.ToMCPSummary(sw, isOwner, ownerPrefs)
		}

		return nil, schema.ListSwitchesOutput{Switches: items, NextCursor: nextCursor}, nil
	}
}

func handleGetSwitch(
	repo repository.SwitchRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.GetSwitchInput, schema.GetSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetSwitchInput) (*mcp.CallToolResult, schema.GetSwitchOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetSwitchOutput{}, err
		}

		sw, err := ownedReadable(ctx, repo.Get, func(sw repository.Switch) repository.Visibility { return sw.Visibility },
			"switch", errSwitchNotFound, log.SwitchID, in.UserID, in.SwitchID)
		if err != nil {
			return nil, schema.GetSwitchOutput{}, err
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.SwitchID, in.SwitchID)
			return nil, schema.GetSwitchOutput{}, errors.New("failed to get switch")
		}

		return nil, schema.GetSwitchOutput{Switch: repomcp.Switch{}.ToMCP(*sw, isOwner, ownerPrefs)}, nil
	}
}

func handleCreateSwitch(
	switchRepo repository.SwitchRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateSwitchInput, schema.CreateSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateSwitchInput) (*mcp.CallToolResult, schema.CreateSwitchOutput, error) {
		sw, err := validatedSwitch(ctx, in.SwitchInput)
		if err != nil {
			return nil, schema.CreateSwitchOutput{}, err
		}

		sw.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.SwitchID, sw.ID)
			return nil, schema.CreateSwitchOutput{}, errors.New("failed to create switch")
		}

		created, err := switchRepo.Create(ctx, sw)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it rather than reporting an internal
			// failure, matching [github.com/roguekeyboards/mykeebs-api/internal/handlers.CreateSwitch]'s 409.
			return nil, schema.CreateSwitchOutput{}, errSwitchAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating switch", log.SwitchID, sw.ID, log.Error, err)
			return nil, schema.CreateSwitchOutput{}, errors.New("failed to create switch")
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.CreateSwitchOutput{Switch: repomcp.Switch{}.ToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateSwitch(
	switchRepo repository.SwitchRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateSwitchInput, schema.UpdateSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateSwitchInput) (*mcp.CallToolResult, schema.UpdateSwitchOutput, error) {
		sw, err := validatedSwitch(ctx, in.SwitchInput)
		if err != nil {
			return nil, schema.UpdateSwitchOutput{}, err
		}

		sw.ID = in.SwitchID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.SwitchID, sw.ID)
			return nil, schema.UpdateSwitchOutput{}, errors.New("failed to update switch")
		}

		updated, err := switchRepo.Update(ctx, sw)
		if mutErr := handleMutationError(ctx, err, log.SwitchID, sw.ID); mutErr != nil {
			return nil, schema.UpdateSwitchOutput{}, mutErr
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.UpdateSwitchOutput{Switch: repomcp.Switch{}.ToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteSwitch(
	switchRepo repository.SwitchRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	switchImages repository.SwitchImageStore,
) mcp.ToolHandlerFor[schema.DeleteSwitchInput, schema.DeleteSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteSwitchInput) (*mcp.CallToolResult, schema.DeleteSwitchOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteSwitchOutput{}, err
		}

		result, err := cascadedelete.DeleteSwitch(ctx, switchRepo, buildRepo, buildImages, switchImages, ownerID, in.SwitchID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteSwitchOutput{}, fmt.Errorf("switch is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting switch", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.DeleteSwitchOutput{}, errors.New("failed to delete switch")
		}

		return nil, schema.DeleteSwitchOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func handleSetSwitchImage(
	switchRepo repository.SwitchRepository,
	images repository.SwitchImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.SetSwitchImageInput, schema.SetSwitchImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.SetSwitchImageInput) (*mcp.CallToolResult, schema.SetSwitchImageOutput, error) {
		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.SetSwitchImageOutput{}, err
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.SetSwitchImageOutput{}, err
		}

		sw, err := switchRepo.Get(ctx, ownerID, in.SwitchID)
		if mutErr := handleMutationError(ctx, err, log.SwitchID, in.SwitchID); mutErr != nil {
			return nil, schema.SetSwitchImageOutput{}, mutErr
		}

		if err := reserveUpload(ctx, uploads, log.SwitchID, in.SwitchID); err != nil {
			return nil, schema.SetSwitchImageOutput{}, err
		}

		key, err := repository.NewSwitchImageKey(ctx, in.SwitchID, uuid.NewString())
		if err != nil {
			log.FromContext(ctx).Error("building switch image key", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.SetSwitchImageOutput{}, errors.New("failed to set switch image")
		}

		uploadURL, err := images.PresignPut(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning switch image upload", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.SetSwitchImageOutput{}, errors.New("failed to set switch image")
		}

		if sw.ImagePath != nil {
			if err := images.Delete(ctx, *sw.ImagePath); err != nil {
				log.FromContext(ctx).Error("deleting previous switch image object", log.SwitchID, in.SwitchID, log.Error, err)
				return nil, schema.SetSwitchImageOutput{}, errors.New("failed to set switch image")
			}
		}

		err = switchRepo.SetImagePath(ctx, in.SwitchID, key)
		if mutErr := handleMutationError(ctx, err, log.SwitchID, in.SwitchID); mutErr != nil {
			return nil, schema.SetSwitchImageOutput{}, mutErr
		}

		return nil, schema.SetSwitchImageOutput{UploadURL: uploadURL}, nil
	}
}

func handleDeleteSwitchImage(
	switchRepo repository.SwitchRepository,
	images repository.SwitchImageStore,
) mcp.ToolHandlerFor[schema.DeleteSwitchImageInput, schema.DeleteSwitchImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteSwitchImageInput) (*mcp.CallToolResult, schema.DeleteSwitchImageOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteSwitchImageOutput{}, err
		}

		sw, err := switchRepo.Get(ctx, ownerID, in.SwitchID)
		if mutErr := handleMutationError(ctx, err, log.SwitchID, in.SwitchID); mutErr != nil {
			return nil, schema.DeleteSwitchImageOutput{}, mutErr
		}

		if sw.ImagePath == nil {
			return nil, schema.DeleteSwitchImageOutput{}, nil
		}

		if err := images.Delete(ctx, *sw.ImagePath); err != nil {
			log.FromContext(ctx).Error("deleting switch image object", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.DeleteSwitchImageOutput{}, errors.New("failed to delete switch image")
		}

		_, err = switchRepo.ClearImagePath(ctx, in.SwitchID)
		if mutErr := handleClearImageError(ctx, err, log.SwitchID, in.SwitchID); mutErr != nil {
			return nil, schema.DeleteSwitchImageOutput{}, mutErr
		}

		return nil, schema.DeleteSwitchImageOutput{}, nil
	}
}

func validatedSwitch(
	ctx context.Context,
	in schema.SwitchInput,
) (repository.Switch, error) {
	sw := repomcp.Switch{}.FromMCP(in)

	if err := lookupErrors(lookup.ValidateSwitch(ctx, sw)); err != nil {
		return repository.Switch{}, err
	}

	return sw, nil
}
