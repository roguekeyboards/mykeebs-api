package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/buildrefs"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/ownerprefs"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// A repository error writes a 500 rather than a 400.
func validateBuildReferences(
	ctx context.Context, w http.ResponseWriter, ownerID string, b repository.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
) (ok bool) {
	fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo)
	if err != nil {
		log.FromContext(ctx).Error("validating build references", log.Error, err)
		problem.Internal(w, "failed to validate build")
		return false
	}
	if len(fieldErrs) > 0 {
		invalidParams := make([]problem.InvalidParam, len(fieldErrs))
		for i, fe := range fieldErrs {
			invalidParams[i] = problem.InvalidParam{Name: fe.Field, Reason: fe.Reason}
		}
		problem.ValidationFailed(w, "one or more fields do not reference resources in your collection", invalidParams)
		return false
	}

	return true
}

// ListBuilds handles GET /v1/users/{userId}/builds. Prices follow
// [repository.ProfilePreferences.ShowPriceSummary], which only ever hides
// prices [repoapi.Build.ToAPI] shows.
func ListBuilds(
	repo repository.BuildRepository,
	br repoapi.Build,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")
		keyboardID := r.URL.Query().Get("keyboard_id")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		builds, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, keyboardID, limit, cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			problem.BadRequest(w, "invalid pagination cursor; restart from the first page")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing builds", log.Error, err)
			problem.Internal(w, "failed to list builds")
			return
		}

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err)
			problem.Internal(w, "failed to list builds")
			return
		}

		items := make([]api.Build, len(builds))
		errs := make([]error, len(builds))

		ctx := r.Context()
		isOwner := authz.IsOwner(ctx, ownerID)
		showPrice := ownerPrefs.ShowPriceSummary(isOwner)
		var wg sync.WaitGroup
		for i, b := range builds {
			wg.Add(1)
			go func(i int, b repository.Build) {
				defer wg.Done()

				item, err := br.ToAPI(ctx, b, isOwner, ownerPrefs)
				if err != nil {
					errs[i] = fmt.Errorf("mapping build %q to API: %w", b.ID, err)
					return
				}
				if !showPrice {
					br.StripPrices(&item)
				}
				items[i] = item
			}(i, b)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(r.Context()).Error("mapping builds to API", log.Error, err)
			problem.Internal(w, "failed to list builds")
			return
		}

		page := api.BuildListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetBuild returns 404, not 403, for a build that exists but isn't
// readable by the caller, to avoid revealing it exists. The owner always
// sees their own price; a non-owner's visibility is gated by the owner's
// Profile preferences - see [repoapi.Build.ToAPI].
func GetBuild(
	repo repository.BuildRepository,
	br repoapi.Build,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		b, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting build", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to get build")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, b.Visibility) {
			log.DeniedRead(r.Context(), "build", ownerID, string(b.Visibility), log.BuildID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		isOwner := authz.IsOwner(r.Context(), ownerID)
		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to get build")
			return
		}

		out, err := br.ToAPI(r.Context(), *b, isOwner, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to get build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// CreateBuild returns 404, not 403, when creating in another user's
// collection, to avoid revealing it exists.
func CreateBuild(
	buildRepo repository.BuildRepository,
	br repoapi.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.BuildInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		b := br.ToRepo(in)

		if !writeLookupErrors(w, lookup.ValidateBuild(r.Context(), b)) {
			return
		}

		if !validateBuildReferences(r.Context(), w, ownerID, b, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo) {
			return
		}

		b.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.BuildID, b.ID)
			problem.Internal(w, "failed to create build")
			return
		}

		created, err := buildRepo.Create(r.Context(), b)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateKeycapSet does.
			problem.Conflict(w, "build already exists")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(r.Context()).Warn("mutation conflict creating build", log.BuildID, b.ID)
			problem.Conflict(w, "the resource is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating build", log.Error, err, log.BuildID, b.ID)
			problem.Internal(w, "failed to create build")
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := br.ToAPI(r.Context(), *created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, created.ID)
			problem.Internal(w, "failed to create build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateBuild reads the {userId} and {buildId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's build, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateBuild(
	buildRepo repository.BuildRepository,
	br repoapi.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	ecPartRepo repository.ECPartRepository,
	keycapSetRepo repository.KeycapSetRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.BuildInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		b := br.ToRepo(in)

		if !writeLookupErrors(w, lookup.ValidateBuild(r.Context(), b)) {
			return
		}

		if !validateBuildReferences(r.Context(), w, ownerID, b, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo) {
			return
		}

		b.ID = id

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.BuildID, b.ID)
			problem.Internal(w, "failed to update build")
			return
		}

		updated, err := buildRepo.Update(r.Context(), b)
		if handleTransactionError(w, r, err, log.BuildID, id) {
			return
		}

		// isOwner: true, already gated above - owner sees price unconditionally.
		out, err := br.ToAPI(r.Context(), *updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, updated.ID)
			problem.Internal(w, "failed to update build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// AddBuildImage reads the {userId} and {buildId} path values and requires
// an authenticated caller. userId must be the caller's own subject; adding
// an image to another user's build, or one that doesn't exist, both return
// 404. Doesn't upload the image itself - the response is a presigned S3 PUT
// URL the client uploads directly to. Presigning runs before the
// repository mutation, so a presign failure never leaves the DB pointing
// at an object that was never uploaded.
// Each issued URL counts against the daily upload limit; once it's
// reached the request gets 429 and no URL is issued.
func AddBuildImage(buildRepo repository.BuildRepository, images repository.BuildImageStore, uploads *uploadlimit.Limiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		buildID := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.CreateBuildImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		if !writeLookupErrors(w, lookup.ValidateImageContentType(r.Context(), in.ContentType)) {
			return
		}

		if reserveUpload(w, r, uploads, log.BuildID, buildID) {
			return
		}

		imageID := uuid.NewString()

		key, err := repository.NewBuildImageKey(r.Context(), buildID, imageID)
		if err != nil {
			log.FromContext(r.Context()).Error("building build image key", log.Error, err, log.BuildID, buildID)
			problem.Internal(w, "failed to add build image")
			return
		}

		uploadURL, err := images.PresignPutBuildImage(r.Context(), key, in.ContentType, in.SizeBytes)
		if err != nil {
			log.FromContext(r.Context()).Error("presigning build image upload", log.Error, err, log.BuildID, buildID)
			problem.Internal(w, "failed to add build image")
			return
		}

		err = buildRepo.AddImage(r.Context(), buildID, repository.BuildImage{ImageID: imageID, Path: key})
		if handleMutationError(w, r, err, log.BuildID, buildID) {
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.BuildImageUpload{ImageId: imageID, UploadUrl: uploadURL})
	}
}

// DeleteBuildImage reads the {userId}, {buildId}, and {imageId} path values
// and requires an authenticated caller. userId must be the caller's own
// subject; removing an image from another user's build always returns 404.
// Idempotent: an imageId not present on the build is not an error. Deletes
// the S3 object before the DB record, so a failure at either step can be
// safely retried to completion: S3's DeleteObject is idempotent, so nothing
// is orphaned and no failure is masked by a false-success retry.
func DeleteBuildImage(buildRepo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		buildID := r.PathValue("buildId")
		imageID := r.PathValue("imageId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		b, err := buildRepo.Get(r.Context(), ownerID, buildID)
		if handleMutationError(w, r, err, log.BuildID, buildID) {
			return
		}

		entry, ok := b.Images[imageID]
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := images.DeleteBuildImage(r.Context(), entry.Path); err != nil {
			log.FromContext(r.Context()).Error("deleting build image object", log.Error, err, log.BuildID, buildID)
			problem.Internal(w, "failed to delete build image")
			return
		}

		_, err = buildRepo.DeleteImage(r.Context(), buildID, imageID)
		if handleClearImageError(w, r, err, log.BuildID, buildID) {
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteBuild reads the {userId} and {buildId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's build always returns 404. Deleting is idempotent: a build
// that doesn't exist returns 204. Deletes every image's S3 object before
// the DB record, aborting before the DB delete if any S3 delete fails,
// same retry-safety reasoning as [DeleteBuildImage].
func DeleteBuild(buildRepo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		ctx := r.Context()

		b, err := buildRepo.Get(ctx, ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err != nil {
			log.FromContext(ctx).Error("getting build", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to delete build")
			return
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
			log.FromContext(ctx).Error("deleting build images", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to delete build")
			return
		}

		err = buildRepo.Delete(ctx, id)
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("mutation conflict deleting build", log.BuildID, id)
			problem.Conflict(w, "the resource is being modified concurrently, please retry")
			return
		}
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(ctx).Error("deleting build", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to delete build")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
