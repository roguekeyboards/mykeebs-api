package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

var errECPartNotFound = errors.New("EC part not found")

var errECPartAlreadyExists = errors.New("EC part already exists")

var listECPartsTool = &mcp.Tool{
	Name:        "list_ec_parts",
	Description: "Lists EC (electro-capacitive, Topre-style) parts in a user's collection: domes, sliders, housings, springs and silencing rings. Set part_type to only list one kind. Returns an abbreviated shape; call get_ec_part for a single part's full details. Omit user_id to list your own parts.",
}

var getECPartTool = &mcp.Tool{
	Name:        "get_ec_part",
	Description: "Returns the full details of one EC part. Omit user_id to read from your own collection.",
}

var createECPartTool = &mcp.Tool{
	Name:        "create_ec_part",
	Description: "Adds an EC part (domes, sliders, housings, springs or silencing rings) to your own collection. Set part_type, and at most the matching one of domes, sliders, housings, springs or rings. Materials and purchase vendor/status must be approved lookup values - call list_lookups and get_lookup to see them.",
}

var updateECPartTool = &mcp.Tool{
	Name:        "update_ec_part",
	Description: "Replaces an EC part in your own collection. Every field is replaced, so omitting an optional field clears it; send the full part, not just the fields you want to change.",
}

var setECPartImageTool = &mcp.Tool{
	Name:        "set_ec_part_image",
	Description: "Mints a presigned URL to upload an EC part's image to. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new image starts pending until it's checked, and only you can see it until it's approved. A rejected image is shown only to you, with the reason, until you replace or delete it.",
}

var deleteECPartImageTool = &mcp.Tool{
	Name:        "delete_ec_part_image",
	Description: "Removes an EC part's image. Idempotent: deleting an EC part's image when it doesn't have one succeeds.",
}

func handleListECParts(
	repo repository.ECPartRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.ListECPartsInput, schema.ListECPartsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListECPartsInput) (*mcp.CallToolResult, schema.ListECPartsOutput, error) {
		partType := repository.ECPartType(in.PartType)
		if partType != "" {
			if err := lookupErrors(lookup.ValidateECPartType(partType)); err != nil {
				return nil, schema.ListECPartsOutput{}, err
			}
		}

		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListECPartsOutput{}, err
		}

		parts, nextCursor, err := repo.List(ctx, ownerID, authz.ReadableVisibilities(ctx, ownerID), partType, clampListLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListECPartsOutput{}, errors.New("invalid cursor; restart from the first page")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing EC parts", log.Error, err)
			return nil, schema.ListECPartsOutput{}, errors.New("failed to list EC parts")
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err)
			return nil, schema.ListECPartsOutput{}, errors.New("failed to list EC parts")
		}

		isOwner := authz.IsOwner(ctx, ownerID)
		items := make([]schema.ECPartSummary, len(parts))
		for i, p := range parts {
			items[i] = repomcp.ECPart{}.ToMCPSummary(p, isOwner, ownerPrefs)
		}

		return nil, schema.ListECPartsOutput{ECParts: items, NextCursor: nextCursor}, nil
	}
}

func handleGetECPart(
	repo repository.ECPartRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.GetECPartInput, schema.GetECPartOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetECPartInput) (*mcp.CallToolResult, schema.GetECPartOutput, error) {
		if strings.TrimSpace(in.ECPartID) == "" {
			return nil, schema.GetECPartOutput{}, errors.New("ec_part_id must not be blank")
		}

		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetECPartOutput{}, err
		}

		p, err := ownedReadable(ctx, repo.Get, func(p repository.ECPart) repository.Visibility { return p.Visibility },
			"EC part", errECPartNotFound, log.ECPartID, in.UserID, in.ECPartID)
		if err != nil {
			return nil, schema.GetECPartOutput{}, err
		}

		ownerPrefs, err := prefs.GetPreferences(ctx, ownerID)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.ECPartID, in.ECPartID)
			return nil, schema.GetECPartOutput{}, errors.New("failed to get EC part")
		}

		return nil, schema.GetECPartOutput{ECPart: repomcp.ECPart{}.ToMCP(*p, authz.IsOwner(ctx, ownerID), ownerPrefs)}, nil
	}
}

func handleCreateECPart(
	repo repository.ECPartRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateECPartInput, schema.CreateECPartOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateECPartInput) (*mcp.CallToolResult, schema.CreateECPartOutput, error) {
		p, err := validatedECPart(ctx, in.ECPartInput)
		if err != nil {
			return nil, schema.CreateECPartOutput{}, err
		}
		p.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.ECPartID, p.ID)
			return nil, schema.CreateECPartOutput{}, errors.New("failed to create EC part")
		}

		created, err := repo.Create(ctx, p)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateECPartOutput{}, errECPartAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating EC part", log.ECPartID, p.ID, log.Error, err)
			return nil, schema.CreateECPartOutput{}, errors.New("failed to create EC part")
		}

		return nil, schema.CreateECPartOutput{ECPart: repomcp.ECPart{}.ToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateECPart(
	repo repository.ECPartRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateECPartInput, schema.UpdateECPartOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateECPartInput) (*mcp.CallToolResult, schema.UpdateECPartOutput, error) {
		if strings.TrimSpace(in.ECPartID) == "" {
			return nil, schema.UpdateECPartOutput{}, errors.New("ec_part_id must not be blank")
		}

		p, err := validatedECPart(ctx, in.ECPartInput)
		if err != nil {
			return nil, schema.UpdateECPartOutput{}, err
		}
		p.ID = in.ECPartID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.ECPartID, p.ID)
			return nil, schema.UpdateECPartOutput{}, errors.New("failed to update EC part")
		}

		updated, err := repo.Update(ctx, p)
		if mutErr := handleMutationError(ctx, err, log.ECPartID, p.ID); mutErr != nil {
			return nil, schema.UpdateECPartOutput{}, mutErr
		}

		return nil, schema.UpdateECPartOutput{ECPart: repomcp.ECPart{}.ToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func validatedECPart(ctx context.Context, in schema.ECPartInput) (repository.ECPart, error) {
	if strings.TrimSpace(in.Brand) == "" {
		return repository.ECPart{}, errors.New("brand must not be blank")
	}
	if strings.TrimSpace(in.Name) == "" {
		return repository.ECPart{}, errors.New("name must not be blank")
	}

	if in.Purchase != nil {
		if err := validatePurchaseDates(in.Purchase.OrderDate, in.Purchase.DeliveryDate); err != nil {
			return repository.ECPart{}, err
		}
	}

	p := repomcp.ECPart{}.FromMCP(in)

	if !p.Visibility.Valid() {
		return repository.ECPart{}, fmt.Errorf(
			"visibility %q must be one of: public, authenticated, private", in.Visibility)
	}

	if err := lookupErrors(lookup.ValidateECPart(ctx, p)); err != nil {
		return repository.ECPart{}, err
	}

	return p, nil
}

func handleSetECPartImage(
	repo repository.ECPartRepository,
	images repository.ECPartImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.SetECPartImageInput, schema.SetECPartImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.SetECPartImageInput) (*mcp.CallToolResult, schema.SetECPartImageOutput, error) {
		if strings.TrimSpace(in.ECPartID) == "" {
			return nil, schema.SetECPartImageOutput{}, errors.New("ec_part_id must not be blank")
		}

		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.SetECPartImageOutput{}, err
		}

		if err := validateImageSize(in.SizeBytes); err != nil {
			return nil, schema.SetECPartImageOutput{}, err
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.SetECPartImageOutput{}, err
		}

		p, err := repo.Get(ctx, ownerID, in.ECPartID)
		if mutErr := handleMutationError(ctx, err, log.ECPartID, in.ECPartID); mutErr != nil {
			return nil, schema.SetECPartImageOutput{}, mutErr
		}

		if err := reserveUpload(ctx, uploads, log.ECPartID, in.ECPartID); err != nil {
			return nil, schema.SetECPartImageOutput{}, err
		}

		key, err := repository.NewECPartImageKey(ctx, in.ECPartID, uuid.NewString())
		if err != nil {
			log.FromContext(ctx).Error("building EC part image key", log.ECPartID, in.ECPartID, log.Error, err)
			return nil, schema.SetECPartImageOutput{}, errors.New("failed to set EC part image")
		}

		uploadURL, err := images.PresignPut(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning EC part image upload", log.ECPartID, in.ECPartID, log.Error, err)
			return nil, schema.SetECPartImageOutput{}, errors.New("failed to set EC part image")
		}

		if p.ImagePath != nil {
			if err := images.Delete(ctx, *p.ImagePath); err != nil {
				log.FromContext(ctx).Error("deleting previous EC part image object", log.ECPartID, in.ECPartID, log.Error, err)
				return nil, schema.SetECPartImageOutput{}, errors.New("failed to set EC part image")
			}
		}

		err = repo.SetImagePath(ctx, in.ECPartID, key)
		if mutErr := handleMutationError(ctx, err, log.ECPartID, in.ECPartID); mutErr != nil {
			return nil, schema.SetECPartImageOutput{}, mutErr
		}

		return nil, schema.SetECPartImageOutput{UploadURL: uploadURL}, nil
	}
}

func handleDeleteECPartImage(
	repo repository.ECPartRepository,
	images repository.ECPartImageStore,
) mcp.ToolHandlerFor[schema.DeleteECPartImageInput, schema.DeleteECPartImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteECPartImageInput) (*mcp.CallToolResult, schema.DeleteECPartImageOutput, error) {
		if strings.TrimSpace(in.ECPartID) == "" {
			return nil, schema.DeleteECPartImageOutput{}, errors.New("ec_part_id must not be blank")
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteECPartImageOutput{}, err
		}

		p, err := repo.Get(ctx, ownerID, in.ECPartID)
		if mutErr := handleMutationError(ctx, err, log.ECPartID, in.ECPartID); mutErr != nil {
			return nil, schema.DeleteECPartImageOutput{}, mutErr
		}

		if p.ImagePath == nil {
			return nil, schema.DeleteECPartImageOutput{}, nil
		}

		if err := images.Delete(ctx, *p.ImagePath); err != nil {
			log.FromContext(ctx).Error("deleting EC part image object", log.ECPartID, in.ECPartID, log.Error, err)
			return nil, schema.DeleteECPartImageOutput{}, errors.New("failed to delete EC part image")
		}

		_, err = repo.ClearImagePath(ctx, in.ECPartID)
		if mutErr := handleClearImageError(ctx, err, log.ECPartID, in.ECPartID); mutErr != nil {
			return nil, schema.DeleteECPartImageOutput{}, mutErr
		}

		return nil, schema.DeleteECPartImageOutput{}, nil
	}
}
