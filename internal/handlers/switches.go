package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

// parseListLimit reads the limit query param. The OpenAPI request validator
// (router.restOpenAPIValidator) enforces range (1-100) and injects the
// spec's default (api/openapi.yaml's Limit param) when absent, so it's
// always present and a valid integer here.
func parseListLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return limit
}

// ListSwitches reads the {userId} path value and lists that owner's
// switches. Anonymous callers are allowed; visibility is scoped to what the
// caller (if any) may read, per [authz.ReadableVisibilities]. Prices follow
// [repository.ProfilePreferences.ShowPriceSummary], which only ever hides
// prices [repoapi.Switch.ToAPI] shows.
func ListSwitches(repo repository.SwitchRepository, sr repoapi.Switch) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		switches, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			problem.BadRequest(w, "invalid pagination cursor; restart from the first page")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing switches", log.Error, err)
			problem.Internal(w, "failed to list switches")
			return
		}

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err)
			problem.Internal(w, "failed to list switches")
			return
		}

		items := make([]api.Switch, len(switches))
		errs := make([]error, len(switches))

		ctx := r.Context()
		isOwner := authz.IsOwner(ctx, ownerID)
		showPrice := ownerPrefs.ShowPriceSummary(isOwner)
		var wg sync.WaitGroup
		for i, sw := range switches {
			wg.Add(1)
			go func(i int, sw repository.Switch) {
				defer wg.Done()

				item, err := sr.ToAPI(ctx, sw, isOwner, ownerPrefs)
				if err != nil {
					errs[i] = fmt.Errorf("mapping switch %q to API: %w", sw.ID, err)
					return
				}
				if !showPrice {
					sr.StripPrices(&item)
				}
				items[i] = item
			}(i, sw)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(r.Context()).Error("mapping switches to API", log.Error, err)
			problem.Internal(w, "failed to list switches")
			return
		}

		page := api.SwitchListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetSwitch reads the {userId} and {switchId} path values. Anonymous callers are
// allowed; a switch that exists but isn't readable by the caller returns
// 404, not 403, to avoid revealing it exists. The owner always sees their
// own price; a non-owner's visibility is gated by the owner's Profile
// preferences - see [repoapi.Switch.ToAPI].
func GetSwitch(repo repository.SwitchRepository, sr repoapi.Switch) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		sw, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to get switch")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, sw.Visibility) {
			log.DeniedRead(r.Context(), "switch", ownerID, string(sw.Visibility), log.SwitchID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		isOwner := authz.IsOwner(r.Context(), ownerID)
		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to get switch")
			return
		}

		out, err := sr.ToAPI(r.Context(), *sw, isOwner, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to get switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

func decodeSwitchInput(w http.ResponseWriter, r *http.Request) (sw repository.Switch, ok bool) {
	var in api.SwitchInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.Switch{}, false
	}

	sw = repoapi.Switch{}.ToRepo(in)

	return sw, true
}

// CreateSwitch reads the {userId} path value and requires an authenticated
// caller. userId must be the caller's own subject; creating in another
// user's collection returns 404, not 403, to avoid revealing it exists.
func CreateSwitch(switchRepo repository.SwitchRepository, sr repoapi.Switch) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		sw, ok := decodeSwitchInput(w, r)
		if !ok {
			return
		}

		if !writeLookupErrors(w, lookup.ValidateSwitch(r.Context(), sw)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.BulkPurchases(sw.Purchases)) {
			return
		}

		sw.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.SwitchID, sw.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		created, err := switchRepo.Create(r.Context(), sw)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless.
			problem.Conflict(w, "switch already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating switch", log.Error, err, log.SwitchID, sw.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := sr.ToAPI(r.Context(), *created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, created.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's switch, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateSwitch(switchRepo repository.SwitchRepository, sr repoapi.Switch) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		sw, ok := decodeSwitchInput(w, r)
		if !ok {
			return
		}

		if !writeLookupErrors(w, lookup.ValidateSwitch(r.Context(), sw)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.BulkPurchases(sw.Purchases)) {
			return
		}

		sw.ID = id

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.SwitchID, sw.ID)
			problem.Internal(w, "failed to update switch")
			return
		}

		updated, err := switchRepo.Update(r.Context(), sw)
		if handleMutationError(w, r, err, log.SwitchID, id) {
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := sr.ToAPI(r.Context(), *updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, updated.ID)
			problem.Internal(w, "failed to update switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's switch returns 404, not 403, to avoid revealing it
// exists. The on_delete query param (default "block") controls what
// happens if a build still references this switch: see
// [cascadedelete.DeleteSwitch], which also handles deleting any image the
// switch (and any cascaded builds) had.
func DeleteSwitch(
	switchRepo repository.SwitchRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	switchImages repository.SwitchImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be block or cascade")
			return
		}

		result, err := cascadedelete.DeleteSwitch(r.Context(), switchRepo, buildRepo, buildImages, switchImages, ownerID, id, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			problem.StillReferenced(w, "switch is still referenced by one or more builds", blocked.BuildIDs)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("deleting switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to delete switch")
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

// SetSwitchImage reads the {userId} and {switchId} path values and
// requires an authenticated caller. userId must be the caller's own
// subject; setting a switch's image on another user's switch, or one
// that doesn't exist, both return 404, to avoid revealing it exists.
// Doesn't upload the image itself - the response is a presigned S3 PUT
// URL the client uploads directly to. Presigning runs before the
// repository mutation, so a presign failure never leaves the DB
// pointing at an object that was never uploaded. Each upload gets a new
// key, so a previous image's object is deleted before the pointer moves,
// S3-first like [DeleteSwitchImage].
// Each issued URL counts against the daily upload limit; once it's
// reached the request gets 429 and no URL is issued.
func SetSwitchImage(switchRepo repository.SwitchRepository, images repository.SwitchImageStore, uploads *uploadlimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.SetSwitchImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		if !writeLookupErrors(w, lookup.ValidateImageContentType(r.Context(), in.ContentType)) {
			return
		}

		sw, err := switchRepo.Get(r.Context(), ownerID, id)
		if handleMutationError(w, r, err, log.SwitchID, id) {
			return
		}

		if reserveUpload(w, r, uploads, log.SwitchID, id) {
			return
		}

		key, err := repository.NewSwitchImageKey(r.Context(), id, uuid.NewString())
		if err != nil {
			log.FromContext(r.Context()).Error("building switch image key", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to set switch image")
			return
		}

		uploadURL, err := images.PresignPut(r.Context(), key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(r.Context()).Error("presigning switch image upload", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to set switch image")
			return
		}

		if sw.ImagePath != nil {
			if err := images.Delete(r.Context(), *sw.ImagePath); err != nil {
				log.FromContext(r.Context()).Error("deleting previous switch image object", log.Error, err, log.SwitchID, id)
				problem.Internal(w, "failed to set switch image")
				return
			}
		}

		err = switchRepo.SetImagePath(r.Context(), id, key)
		if handleMutationError(w, r, err, log.SwitchID, id) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.SwitchImageUpload{UploadUrl: uploadURL})
	}
}

// DeleteSwitchImage reads the {userId} and {switchId} path values and
// requires an authenticated caller. userId must be the caller's own
// subject; removing a switch's image on another user's switch, or one
// that doesn't exist, both return 404, to avoid revealing it exists.
// Idempotent: a switch with no image already set is not an error. Deletes
// the S3 object before the DB record, same retry-safety reasoning as
// [cascadedelete.DeleteSwitch].
func DeleteSwitchImage(switchRepo repository.SwitchRepository, images repository.SwitchImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		sw, err := switchRepo.Get(r.Context(), ownerID, id)
		if handleMutationError(w, r, err, log.SwitchID, id) {
			return
		}

		if sw.ImagePath == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := images.Delete(r.Context(), *sw.ImagePath); err != nil {
			log.FromContext(r.Context()).Error("deleting switch image object", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to delete switch image")
			return
		}

		_, err = switchRepo.ClearImagePath(r.Context(), id)
		if handleClearImageError(w, r, err, log.SwitchID, id) {
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
