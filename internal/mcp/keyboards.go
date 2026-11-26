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
	"github.com/roguekeyboards/mykeebs-api/internal/purchasevalidate"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

var errKeyboardNotFound = errors.New("keyboard not found")

var errKeyboardAlreadyExists = errors.New("keyboard already exists")

var listKeyboardsTool = &mcp.Tool{
	Name:        "list_keyboards",
	Description: "Lists keyboards in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_keyboard for a single keyboard's full details. Omit user_id to list your own keyboards.",
}

var getKeyboardTool = &mcp.Tool{
	Name:        "get_keyboard",
	Description: "Returns the full details of one keyboard, including its case design, plates, PCBs, purchase history and total_cost (the base purchase price plus every plate's and PCB's price). Omit user_id to read from your own collection.",
}

var createKeyboardTool = &mcp.Tool{
	Name:        "create_keyboard",
	Description: "Adds a keyboard to your own collection. Add its plates and PCBs afterwards with create_keyboard_plate and create_keyboard_pcb. The purchase price is the base price, excluding any plate or PCB that gets its own purchase. size, layout, the design materials and the purchase vendor/status must be approved lookup values - call list_lookups and get_lookup to see them. layout must additionally be valid for the chosen size; get_lookup(\"keyboard_layout\") lists the sizes each layout allows.",
}

var updateKeyboardTool = &mcp.Tool{
	Name:        "update_keyboard",
	Description: "Replaces a keyboard in your own collection. Every field is replaced, so omitting an optional field clears it; send the full keyboard, not just the fields you want to change. Plates, PCBs and images are left untouched - manage them with their own tools.",
}

var deleteKeyboardTool = &mcp.Tool{
	Name:        "delete_keyboard",
	Description: "Removes a keyboard from your own collection. Idempotent: deleting a keyboard that isn't there succeeds. on_delete controls what happens if a build still references this keyboard: \"block\" (default) fails and lists the blocking build ids; \"cascade\" deletes the keyboard and every referencing build.",
}

var listKeyboardImagesTool = &mcp.Tool{
	Name:        "list_keyboard_images",
	Description: "Lists the ids of a keyboard's images. get_keyboard/list_keyboards only report whether any exist via has_images - call this to get their ids, e.g. before deleting one. Its owner gets every image with its moderation status; anyone else gets approved images only.",
}

var addKeyboardImageTool = &mcp.Tool{
	Name:        "add_keyboard_image",
	Description: "Adds an image to a keyboard in your own collection. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new image starts pending until it's checked, and only you can see it until it's approved. A rejected image is shown only to you, with the reason, and still counts toward the image limit until you delete it. A keyboard may have at most 10 images.",
}

var deleteKeyboardImageTool = &mcp.Tool{
	Name:        "delete_keyboard_image",
	Description: "Removes an image from a keyboard. Idempotent: deleting an image that isn't there succeeds.",
}

func handleListKeyboards(
	repo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.ListKeyboardsInput, schema.ListKeyboardsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeyboardsInput) (*mcp.CallToolResult, schema.ListKeyboardsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListKeyboardsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		keyboards, nextCursor, err := repo.List(ctx, ownerID, visibilities, listLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListKeyboardsOutput{}, errors.New("invalid cursor; restart from the first page")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing keyboards", log.Error, err)
			return nil, schema.ListKeyboardsOutput{}, errors.New("failed to list keyboards")
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err)
			return nil, schema.ListKeyboardsOutput{}, errors.New("failed to list keyboards")
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		items := make([]schema.KeyboardSummary, len(keyboards))
		for i, kb := range keyboards {
			items[i] = repomcp.Keyboard{}.ToMCPSummary(kb, isOwner, ownerPrefs)
		}

		return nil, schema.ListKeyboardsOutput{Keyboards: items, NextCursor: nextCursor}, nil
	}
}

func handleGetKeyboard(
	repo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.GetKeyboardInput, schema.GetKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeyboardInput) (*mcp.CallToolResult, schema.GetKeyboardOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		kb, err := ownedReadable(ctx, repo.Get, func(k repository.Keyboard) repository.Visibility { return k.Visibility },
			"keyboard", errKeyboardNotFound, log.KeyboardID, in.UserID, in.KeyboardID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, in.KeyboardID)
			return nil, schema.GetKeyboardOutput{}, errors.New("failed to get keyboard")
		}

		return nil, schema.GetKeyboardOutput{Keyboard: repomcp.Keyboard{}.ToMCP(*kb, isOwner, ownerPrefs)}, nil
	}
}

func handleCreateKeyboard(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateKeyboardInput, schema.CreateKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeyboardInput) (*mcp.CallToolResult, schema.CreateKeyboardOutput, error) {
		kb, err := validatedKeyboard(ctx, in.KeyboardInput)
		if err != nil {
			return nil, schema.CreateKeyboardOutput{}, err
		}

		kb.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, kb.ID)
			return nil, schema.CreateKeyboardOutput{}, errors.New("failed to create keyboard")
		}

		created, err := keyboardRepo.Create(ctx, kb)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateKeyboardOutput{}, errKeyboardAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating keyboard", log.KeyboardID, kb.ID, log.Error, err)
			return nil, schema.CreateKeyboardOutput{}, errors.New("failed to create keyboard")
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.CreateKeyboardOutput{Keyboard: repomcp.Keyboard{}.ToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateKeyboard(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateKeyboardInput, schema.UpdateKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeyboardInput) (*mcp.CallToolResult, schema.UpdateKeyboardOutput, error) {
		kb, err := validatedKeyboard(ctx, in.KeyboardInput)
		if err != nil {
			return nil, schema.UpdateKeyboardOutput{}, err
		}

		kb.ID = in.KeyboardID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, kb.ID)
			return nil, schema.UpdateKeyboardOutput{}, errors.New("failed to update keyboard")
		}

		updated, err := keyboardRepo.Update(ctx, kb)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, kb.ID); mutErr != nil {
			return nil, schema.UpdateKeyboardOutput{}, mutErr
		}

		// isOwner: true, this always targets the caller's own collection.
		return nil, schema.UpdateKeyboardOutput{Keyboard: repomcp.Keyboard{}.ToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteKeyboard(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	keyboardImages repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardInput, schema.DeleteKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardInput) (*mcp.CallToolResult, schema.DeleteKeyboardOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeyboardOutput{}, err
		}

		result, err := cascadedelete.DeleteKeyboard(ctx, keyboardRepo, buildRepo, buildImages, keyboardImages, ownerID, in.KeyboardID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteKeyboardOutput{}, fmt.Errorf("keyboard is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting keyboard", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.DeleteKeyboardOutput{}, errors.New("failed to delete keyboard")
		}

		return nil, schema.DeleteKeyboardOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func handleListKeyboardImages(
	repo repository.KeyboardRepository,
) mcp.ToolHandlerFor[schema.ListKeyboardImagesInput, schema.ListKeyboardImagesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeyboardImagesInput) (*mcp.CallToolResult, schema.ListKeyboardImagesOutput, error) {
		kb, err := ownedReadable(ctx, repo.Get, func(k repository.Keyboard) repository.Visibility { return k.Visibility },
			"keyboard", errKeyboardNotFound, log.KeyboardID, in.UserID, in.KeyboardID)
		if err != nil {
			return nil, schema.ListKeyboardImagesOutput{}, err
		}

		images := repomcp.Keyboard{}.ImagesToMCP(*kb, authz.IsOwner(ctx, kb.UserID))

		return nil, schema.ListKeyboardImagesOutput{Images: images}, nil
	}
}

func handleAddKeyboardImage(
	keyboardRepo repository.KeyboardRepository,
	images repository.KeyboardImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.AddKeyboardImageInput, schema.AddKeyboardImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.AddKeyboardImageInput) (*mcp.CallToolResult, schema.AddKeyboardImageOutput, error) {
		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.AddKeyboardImageOutput{}, err
		}

		if err := reserveUpload(ctx, uploads, log.KeyboardID, in.KeyboardID); err != nil {
			return nil, schema.AddKeyboardImageOutput{}, err
		}

		imageID := uuid.NewString()

		key, err := repository.NewKeyboardImageKey(ctx, in.KeyboardID, imageID)
		if err != nil {
			log.FromContext(ctx).Error("building keyboard image key", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.AddKeyboardImageOutput{}, errors.New("failed to add keyboard image")
		}

		uploadURL, err := images.PresignPutKeyboardImage(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning keyboard image upload", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.AddKeyboardImageOutput{}, errors.New("failed to add keyboard image")
		}

		err = keyboardRepo.AddImage(ctx, in.KeyboardID, repository.KeyboardImage{ImageID: imageID, Path: key})
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID); mutErr != nil {
			return nil, schema.AddKeyboardImageOutput{}, mutErr
		}

		return nil, schema.AddKeyboardImageOutput{ImageID: imageID, UploadURL: uploadURL}, nil
	}
}

func handleDeleteKeyboardImage(
	keyboardRepo repository.KeyboardRepository,
	images repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardImageInput, schema.DeleteKeyboardImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardImageInput) (*mcp.CallToolResult, schema.DeleteKeyboardImageOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeyboardImageOutput{}, err
		}

		kb, err := keyboardRepo.Get(ctx, ownerID, in.KeyboardID)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID); mutErr != nil {
			return nil, schema.DeleteKeyboardImageOutput{}, mutErr
		}

		entry, ok := kb.Images[in.ImageID]
		if !ok {
			return nil, schema.DeleteKeyboardImageOutput{}, nil
		}

		if err := images.DeleteKeyboardImage(ctx, entry.Path); err != nil {
			log.FromContext(ctx).Error("deleting keyboard image object", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.DeleteKeyboardImageOutput{}, errors.New("failed to delete keyboard image")
		}

		_, err = keyboardRepo.DeleteImage(ctx, in.KeyboardID, in.ImageID)
		if mutErr := handleClearImageError(ctx, err, log.KeyboardID, in.KeyboardID); mutErr != nil {
			return nil, schema.DeleteKeyboardImageOutput{}, mutErr
		}

		return nil, schema.DeleteKeyboardImageOutput{}, nil
	}
}

func validatedKeyboard(
	ctx context.Context,
	in schema.KeyboardInput,
) (repository.Keyboard, error) {
	kb := repomcp.Keyboard{}.FromMCP(in)

	if err := lookupErrors(lookup.ValidateKeyboard(ctx, kb)); err != nil {
		return repository.Keyboard{}, err
	}
	if err := purchaseErrors(purchasevalidate.KeyboardPurchase(kb.Purchase)); err != nil {
		return repository.Keyboard{}, err
	}

	return kb, nil
}
