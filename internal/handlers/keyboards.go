package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/cascadedelete"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/ownerprefs"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/purchasevalidate"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// ListKeyboards reads the {userId} path value and lists that owner's
// keyboards. Anonymous callers are allowed; visibility is scoped to what
// the caller (if any) may read, per [authz.ReadableVisibilities]. Prices
// follow [repository.ProfilePreferences.ShowPriceSummary], which only ever
// hides prices [repoapi.Keyboard.ToAPI] shows.
func ListKeyboards(repo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		keyboards, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			problem.BadRequest(w, "invalid pagination cursor; restart from the first page")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing keyboards", log.Error, err)
			problem.Internal(w, "failed to list keyboards")
			return
		}

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err)
			problem.Internal(w, "failed to list keyboards")
			return
		}

		items := make([]api.Keyboard, len(keyboards))
		errs := make([]error, len(keyboards))

		ctx := r.Context()
		isOwner := authz.IsOwner(ctx, ownerID)
		showPrice := ownerPrefs.ShowPriceSummary(isOwner)
		var wg sync.WaitGroup
		for i, kb := range keyboards {
			wg.Add(1)
			go func(i int, kb repository.Keyboard) {
				defer wg.Done()

				item, err := kr.ToAPI(ctx, kb, isOwner, ownerPrefs)
				if err != nil {
					errs[i] = fmt.Errorf("mapping keyboard %q to API: %w", kb.ID, err)
					return
				}
				if !showPrice {
					kr.StripPrices(&item)
				}
				items[i] = item
			}(i, kb)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(r.Context()).Error("mapping keyboards to API", log.Error, err)
			problem.Internal(w, "failed to list keyboards")
			return
		}

		page := api.KeyboardListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetKeyboard reads the {userId} and {keyboardId} path values. Anonymous callers
// are allowed; a keyboard that exists but isn't readable by the caller
// returns 404, not 403, to avoid revealing it exists. The owner always
// sees their own price; a non-owner's visibility is gated by the owner's
// Profile preferences - see [repoapi.Keyboard.ToAPI].
func GetKeyboard(repo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		kb, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting keyboard", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, kb.Visibility) {
			log.DeniedRead(r.Context(), "keyboard", ownerID, string(kb.Visibility), log.KeyboardID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		isOwner := authz.IsOwner(r.Context(), ownerID)
		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		out, err := kr.ToAPI(r.Context(), *kb, isOwner, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

func decodeKeyboardInput(w http.ResponseWriter, r *http.Request) (kb repository.Keyboard, ok bool) {
	var in api.KeyboardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.Keyboard{}, false
	}

	kb = repoapi.Keyboard{}.ToRepo(in)

	return kb, true
}

// CreateKeyboard reads the {userId} path value and requires an
// authenticated caller. userId must be the caller's own subject; creating
// in another user's collection returns 404, not 403, to avoid revealing it
// exists.
func CreateKeyboard(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		kb, ok := decodeKeyboardInput(w, r)
		if !ok {
			return
		}

		if !writeLookupErrors(w, lookup.ValidateKeyboard(r.Context(), kb)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(kb.Purchase)) {
			return
		}

		kb.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, kb.ID)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		created, err := keyboardRepo.Create(r.Context(), kb)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateSwitch does.
			problem.Conflict(w, "keyboard already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating keyboard", log.Error, err, log.KeyboardID, kb.ID)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := kr.ToAPI(r.Context(), *created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, created.ID)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateKeyboard reads the {userId} and {keyboardId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's keyboard, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateKeyboard(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		kb, ok := decodeKeyboardInput(w, r)
		if !ok {
			return
		}

		if !writeLookupErrors(w, lookup.ValidateKeyboard(r.Context(), kb)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(kb.Purchase)) {
			return
		}

		kb.ID = id

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, kb.ID)
			problem.Internal(w, "failed to update keyboard")
			return
		}

		updated, err := keyboardRepo.Update(r.Context(), kb)
		if handleMutationError(w, r, err, log.KeyboardID, id) {
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := kr.ToAPI(r.Context(), *updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, updated.ID)
			problem.Internal(w, "failed to update keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteKeyboard reads the {userId} and {keyboardId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's keyboard returns 404, not 403, to avoid revealing it
// exists. The on_delete query param (default "block") controls what
// happens if a build still references this keyboard: see
// [cascadedelete.DeleteKeyboard], which also handles deleting any images
// the keyboard (and any cascaded builds) had.
func DeleteKeyboard(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	keyboardImages repository.KeyboardImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be block or cascade")
			return
		}

		result, err := cascadedelete.DeleteKeyboard(r.Context(), keyboardRepo, buildRepo, buildImages, keyboardImages, ownerID, id, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			problem.StillReferenced(w, "keyboard is still referenced by one or more builds", blocked.BuildIDs)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("deleting keyboard", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to delete keyboard")
			return
		}

		if onDelete == cascadedelete.OnDeleteCascade {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.CascadeDeleteResult{DeletedBuildIds: result.DeletedBuildIDs})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// AddKeyboardImage reads the {userId} and {keyboardId} path values and
// requires an authenticated caller. userId must be the caller's own
// subject; adding an image to another user's keyboard, or one that
// doesn't exist, both return 404. Doesn't upload the image itself - the
// response is a presigned S3 PUT URL the client uploads directly to.
// Presigning runs before the repository mutation, so a presign failure
// never leaves the DB pointing at an object that was never uploaded.
// Each issued URL counts against the daily upload limit; once it's
// reached the request gets 429 and no URL is issued.
func AddKeyboardImage(keyboardRepo repository.KeyboardRepository, images repository.KeyboardImageStore, uploads *uploadlimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.CreateKeyboardImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		if !writeLookupErrors(w, lookup.ValidateImageContentType(r.Context(), in.ContentType)) {
			return
		}

		if reserveUpload(w, r, uploads, log.KeyboardID, keyboardID) {
			return
		}

		imageID := uuid.NewString()

		key, err := repository.NewKeyboardImageKey(r.Context(), keyboardID, imageID)
		if err != nil {
			log.FromContext(r.Context()).Error("building keyboard image key", log.Error, err, log.KeyboardID, keyboardID)
			problem.Internal(w, "failed to add keyboard image")
			return
		}

		uploadURL, err := images.PresignPutKeyboardImage(r.Context(), key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(r.Context()).Error("presigning keyboard image upload", log.Error, err, log.KeyboardID, keyboardID)
			problem.Internal(w, "failed to add keyboard image")
			return
		}

		err = keyboardRepo.AddImage(r.Context(), keyboardID, repository.KeyboardImage{ImageID: imageID, Path: key})
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.KeyboardImageUpload{ImageId: imageID, UploadUrl: uploadURL})
	}
}

// DeleteKeyboardImage reads the {userId}, {keyboardId}, and {imageId} path
// values and requires an authenticated caller. userId must be the
// caller's own subject; removing an image from another user's keyboard
// always returns 404. Idempotent: an imageId not present on the keyboard
// is not an error. Deletes the S3 object before the DB record, same
// retry-safety reasoning as [cascadedelete.DeleteKeyboard].
func DeleteKeyboardImage(keyboardRepo repository.KeyboardRepository, images repository.KeyboardImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")
		imageID := r.PathValue("imageId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		kb, err := keyboardRepo.Get(r.Context(), ownerID, keyboardID)
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID) {
			return
		}

		entry, ok := kb.Images[imageID]
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := images.DeleteKeyboardImage(r.Context(), entry.Path); err != nil {
			log.FromContext(r.Context()).Error("deleting keyboard image object", log.Error, err, log.KeyboardID, keyboardID)
			problem.Internal(w, "failed to delete keyboard image")
			return
		}

		_, err = keyboardRepo.DeleteImage(r.Context(), keyboardID, imageID)
		if handleClearImageError(w, r, err, log.KeyboardID, keyboardID) {
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
