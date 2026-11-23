package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/profileread"
	"github.com/roguekeyboards/mykeebs-api/internal/profilevalidate"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

var errProfileNotFound = errors.New("profile not found")

var errProfileAlreadyExists = errors.New("you already have a profile")

var getProfileTool = &mcp.Tool{
	Name:        "get_profile",
	Description: "Returns one user's public profile. identifier is either the user's id or their username. A profile that isn't discoverable is only visible to its owner; pass your own user id to read your own non-discoverable profile.",
}

var createProfileTool = &mcp.Tool{
	Name:        "create_profile",
	Description: "Creates your own public profile. You may have only one profile - creating a second fails. username must be 3-32 chars of lowercase letters, digits, hyphen, period, or underscore, must not start or end with a period, hyphen, or underscore, must not contain consecutive periods, and must be unique across all users. Any Discord handle (3+ chars) is a valid username. Omitting an optional field (discord_username, bio, links) leaves it unset.",
}

var updateProfileTool = &mcp.Tool{
	Name:        "update_profile",
	Description: "Replaces your own public profile. This is a full replace: every field is overwritten, so omitting bio or links clears them - send the complete profile, not just the fields you want to change. The avatar is not part of this call and is left untouched. Fails if you have no profile yet, or if the requested username is already taken by another user.",
}

var deleteProfileTool = &mcp.Tool{
	Name:        "delete_profile",
	Description: "Deletes your whole account: your profile, avatar and username, and every build, keyboard, switch, EC part and keycap set you own, with all their images. This can't be undone. Safe to retry: if it fails partway, calling it again deletes the rest. Succeeds when there's nothing left to delete.",
}

var listProfilesTool = &mcp.Tool{
	Name:        "list_profiles",
	Description: "Lists discoverable profiles in the public directory, ordered by username. Returns an abbreviated shape; call get_profile with a row's user_id for a profile's full details. username and discord_username are optional begins-with prefix filters and are mutually exclusive - pass at most one.",
}

var setProfileImageTool = &mcp.Tool{
	Name:        "set_profile_image",
	Description: "Mints a presigned URL to upload your profile's avatar to. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header, an If-None-Match: * header, and a body of exactly size_bytes. The upload_url is for one upload: another PUT to it fails with 412 while that image is stored. The image must be a WebP, at most 2560px on its longest side and at most 1.5 MB: before calling this, check the image and convert it to WebP and resize it if it isn't, or tell the user it can't be uploaded. An upload that isn't really a WebP, or is over 2560px, is rejected. If image uploads aren't available at the moment, this fails without issuing a URL; try again later. A new avatar starts pending until it's checked, and only you can see it until it's approved. A rejected avatar is shown only to you, with the reason, until you replace or delete it. Your profile has at most one avatar; calling this again replaces it. Fails if you have no profile yet.",
}

var deleteProfileImageTool = &mcp.Tool{
	Name:        "delete_profile_image",
	Description: "Removes your profile's avatar. Idempotent: deleting when your profile has no avatar succeeds. Fails if you have no profile.",
}

func handleGetProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.GetProfileInput, schema.GetProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetProfileInput) (*mcp.CallToolResult, schema.GetProfileOutput, error) {
		p, ok, err := profileread.Resolve(ctx, repo, in.Identifier)
		if err != nil {
			log.FromContext(ctx).Error("getting profile", log.Error, err, log.ProfileID, in.Identifier)
			return nil, schema.GetProfileOutput{}, errors.New("failed to get profile")
		}
		if !ok {
			return nil, schema.GetProfileOutput{}, errProfileNotFound
		}

		return nil, schema.GetProfileOutput{Profile: repomcp.Profile{}.ToMCP(*p, authz.IsOwner(ctx, p.OwnerID))}, nil
	}
}

func handleCreateProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.CreateProfileInput, schema.CreateProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateProfileInput) (*mcp.CallToolResult, schema.CreateProfileOutput, error) {
		p, err := validatedProfile(in.ProfileInput)
		if err != nil {
			return nil, schema.CreateProfileOutput{}, err
		}

		created, err := repo.Create(ctx, p)
		switch {
		case errors.Is(err, repository.ErrAlreadyExists):
			return nil, schema.CreateProfileOutput{}, errProfileAlreadyExists
		case errors.Is(err, repository.ErrUsernameTaken):
			return nil, schema.CreateProfileOutput{}, fmt.Errorf("username %q is already taken", p.Username)
		case errors.Is(err, repository.ErrMutationConflict):
			return nil, schema.CreateProfileOutput{}, handleTransactionError(ctx, err)
		case err != nil:
			log.FromContext(ctx).Error("creating profile", log.Error, err)
			return nil, schema.CreateProfileOutput{}, errors.New("failed to create profile")
		}

		return nil, schema.CreateProfileOutput{Profile: repomcp.Profile{}.ToMCP(*created, true)}, nil
	}
}

func handleUpdateProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.UpdateProfileInput, schema.UpdateProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateProfileInput) (*mcp.CallToolResult, schema.UpdateProfileOutput, error) {
		p, err := validatedProfile(in.ProfileInput)
		if err != nil {
			return nil, schema.UpdateProfileOutput{}, err
		}

		updated, err := repo.Update(ctx, p)
		if errors.Is(err, repository.ErrUsernameTaken) {
			return nil, schema.UpdateProfileOutput{}, fmt.Errorf("username %q is already taken", p.Username)
		}
		if mutErr := handleTransactionError(ctx, err); mutErr != nil {
			return nil, schema.UpdateProfileOutput{}, mutErr
		}

		return nil, schema.UpdateProfileOutput{Profile: repomcp.Profile{}.ToMCP(*updated, true)}, nil
	}
}

func handleDeleteProfile(account accountdelete.Account) mcp.ToolHandlerFor[schema.DeleteProfileInput, schema.DeleteProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ schema.DeleteProfileInput) (*mcp.CallToolResult, schema.DeleteProfileOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteProfileOutput{}, err
		}

		if mutErr := handleTransactionError(ctx, account.Delete(ctx), log.ProfileID, ownerID); mutErr != nil {
			return nil, schema.DeleteProfileOutput{}, mutErr
		}

		return nil, schema.DeleteProfileOutput{}, nil
	}
}

func handleListProfiles(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.ListProfilesInput, schema.ListProfilesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListProfilesInput) (*mcp.CallToolResult, schema.ListProfilesOutput, error) {
		profiles, nextCursor, err := repo.ListPublic(ctx, in.Username, in.DiscordUsername, listLimit(in.Limit), in.Cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			return nil, schema.ListProfilesOutput{}, errors.New("invalid cursor; restart from the first page (a cursor can't be reused with a different filter)")
		}
		if err != nil {
			log.FromContext(ctx).Error("listing profiles", log.Error, err)
			return nil, schema.ListProfilesOutput{}, errors.New("failed to list profiles")
		}

		items := make([]schema.ProfileSummary, len(profiles))
		for i, p := range profiles {
			items[i] = repomcp.Profile{}.ToMCPSummary(p, authz.IsOwner(ctx, p.OwnerID))
		}

		return nil, schema.ListProfilesOutput{Profiles: items, NextCursor: nextCursor}, nil
	}
}

func handleSetProfileImage(
	repo repository.ProfileRepository,
	images repository.ProfileImageStore,
	uploads *uploadlimit.Limiter,
) mcp.ToolHandlerFor[schema.SetProfileImageInput, schema.SetProfileImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.SetProfileImageInput) (*mcp.CallToolResult, schema.SetProfileImageOutput, error) {
		if err := lookupErrors(lookup.ValidateImageContentType(ctx, in.ContentType)); err != nil {
			return nil, schema.SetProfileImageOutput{}, err
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.SetProfileImageOutput{}, err
		}

		p, err := repo.Get(ctx, ownerID)
		if mutErr := handleMutationError(ctx, err, log.ProfileID, ownerID); mutErr != nil {
			return nil, schema.SetProfileImageOutput{}, mutErr
		}

		if err := reserveUpload(ctx, uploads, log.ProfileID, ownerID); err != nil {
			return nil, schema.SetProfileImageOutput{}, err
		}

		key, err := repository.NewProfileImageKey(ctx, uuid.NewString())
		if err != nil {
			log.FromContext(ctx).Error("building profile image key", log.Error, err, log.ProfileID, ownerID)
			return nil, schema.SetProfileImageOutput{}, errors.New("failed to set profile image")
		}

		uploadURL, err := images.PresignPut(ctx, key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(ctx).Error("presigning profile image upload", log.Error, err, log.ProfileID, ownerID)
			return nil, schema.SetProfileImageOutput{}, errors.New("failed to set profile image")
		}

		if p.AvatarPath != nil {
			if err := images.Delete(ctx, *p.AvatarPath); err != nil {
				log.FromContext(ctx).Error("deleting previous profile avatar object", log.Error, err, log.ProfileID, ownerID)
				return nil, schema.SetProfileImageOutput{}, errors.New("failed to set profile image")
			}
		}

		if mutErr := handleMutationError(ctx, repo.SetAvatarPath(ctx, key)); mutErr != nil {
			return nil, schema.SetProfileImageOutput{}, mutErr
		}

		return nil, schema.SetProfileImageOutput{UploadURL: uploadURL}, nil
	}
}

func handleDeleteProfileImage(
	repo repository.ProfileRepository,
	images repository.ProfileImageStore,
) mcp.ToolHandlerFor[schema.DeleteProfileImageInput, schema.DeleteProfileImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ schema.DeleteProfileImageInput) (*mcp.CallToolResult, schema.DeleteProfileImageOutput, error) {
		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteProfileImageOutput{}, err
		}

		p, err := repo.Get(ctx, ownerID)
		if mutErr := handleMutationError(ctx, err, log.ProfileID, ownerID); mutErr != nil {
			return nil, schema.DeleteProfileImageOutput{}, mutErr
		}

		if p.AvatarPath == nil {
			return nil, schema.DeleteProfileImageOutput{}, nil
		}

		if err := images.Delete(ctx, *p.AvatarPath); err != nil {
			log.FromContext(ctx).Error("deleting profile avatar object", log.Error, err, log.ProfileID, ownerID)
			return nil, schema.DeleteProfileImageOutput{}, errors.New("failed to delete profile image")
		}

		_, err = repo.ClearAvatarPath(ctx)
		if mutErr := handleClearImageError(ctx, err, log.ProfileID, ownerID); mutErr != nil {
			return nil, schema.DeleteProfileImageOutput{}, mutErr
		}

		return nil, schema.DeleteProfileImageOutput{}, nil
	}
}

// validatedProfile runs the full field rule set (the SDK enforces none of
// it) and maps the input to a repository.Profile, joining every violation
// into one error.
func validatedProfile(in schema.ProfileInput) (repository.Profile, error) {
	p := repomcp.Profile{}.FromMCP(in)

	profilevalidate.Normalize(&p)
	fieldErrs := profilevalidate.Validate(p)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = fe.Name + ": " + fe.Reason
		}

		return repository.Profile{}, errors.New(strings.Join(reasons, "; "))
	}

	return p, nil
}
