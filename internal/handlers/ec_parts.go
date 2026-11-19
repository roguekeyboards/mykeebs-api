package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/roguekeyboards/mykeebs-api/internal/authz"
	"github.com/roguekeyboards/mykeebs-api/internal/handlers/api"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/ownerprefs"
	"github.com/roguekeyboards/mykeebs-api/internal/problem"
	"github.com/roguekeyboards/mykeebs-api/internal/repoapi"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

// ListECParts reads the {userId} path value and lists that owner's EC
// parts, optionally only those of the part_type query param's type.
// Anonymous callers are allowed; visibility is scoped to what the caller
// (if any) may read, per [authz.ReadableVisibilities]. Prices follow
// [repository.ProfilePreferences.ShowPriceSummary].
func ListECParts(repo repository.ECPartRepository, er repoapi.ECPart) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		partType := repository.ECPartType(r.URL.Query().Get("part_type"))

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		parts, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, partType, parseListLimit(r), r.URL.Query().Get("cursor"))
		if errors.Is(err, repository.ErrInvalidCursor) {
			problem.BadRequest(w, "invalid pagination cursor; restart from the first page")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing EC parts", log.Error, err)
			problem.Internal(w, "failed to list EC parts")
			return
		}

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err)
			problem.Internal(w, "failed to list EC parts")
			return
		}

		isOwner := authz.IsOwner(r.Context(), ownerID)
		showPrice := ownerPrefs.ShowPriceSummary(isOwner)
		items := make([]api.ECPart, len(parts))
		for i, p := range parts {
			item, err := er.ToAPI(p, isOwner, ownerPrefs)
			if err != nil {
				log.FromContext(r.Context()).Error("mapping EC part to API", log.Error, err, log.ECPartID, p.ID)
				problem.Internal(w, "failed to list EC parts")
				return
			}
			if !showPrice {
				er.StripPrices(&item)
			}
			items[i] = item
		}

		page := api.ECPartListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetECPart reads the {userId} and {ecPartId} path values. Anonymous
// callers are allowed; a part that exists but isn't readable by the caller
// returns 404, not 403, to avoid revealing it exists.
func GetECPart(repo repository.ECPartRepository, er repoapi.ECPart) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("ecPartId")

		p, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting EC part", log.Error, err, log.ECPartID, id)
			problem.Internal(w, "failed to get EC part")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, p.Visibility) {
			log.DeniedRead(r.Context(), "EC part", ownerID, string(p.Visibility), log.ECPartID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.ECPartID, id)
			problem.Internal(w, "failed to get EC part")
			return
		}

		out, err := er.ToAPI(*p, authz.IsOwner(r.Context(), ownerID), ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping EC part to API", log.Error, err, log.ECPartID, id)
			problem.Internal(w, "failed to get EC part")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

func decodeECPartInput(w http.ResponseWriter, r *http.Request) (repository.ECPart, bool) {
	var in api.ECPartInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.ECPart{}, false
	}

	p := repoapi.ECPart{}.ToRepo(in)
	if !writeLookupErrors(w, lookup.ValidateECPart(r.Context(), p)) {
		return repository.ECPart{}, false
	}

	return p, true
}

// CreateECPart reads the {userId} path value and requires an authenticated
// caller. userId must be the caller's own subject; creating in another
// user's collection returns 404, not 403, to avoid revealing it exists.
func CreateECPart(repo repository.ECPartRepository, er repoapi.ECPart) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !authz.IsOwner(r.Context(), r.PathValue("userId")) {
			problem.NotFound(w, "resource not found")
			return
		}

		p, ok := decodeECPartInput(w, r)
		if !ok {
			return
		}
		p.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.ECPartID, p.ID)
			problem.Internal(w, "failed to create EC part")
			return
		}

		created, err := repo.Create(r.Context(), p)
		if errors.Is(err, repository.ErrAlreadyExists) {
			problem.Conflict(w, "EC part already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating EC part", log.Error, err, log.ECPartID, p.ID)
			problem.Internal(w, "failed to create EC part")
			return
		}

		out, err := er.ToAPI(*created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping EC part to API", log.Error, err, log.ECPartID, created.ID)
			problem.Internal(w, "failed to create EC part")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateECPart reads the {userId} and {ecPartId} path values and requires
// an authenticated caller. userId must be the caller's own subject;
// updating another user's part, or one that doesn't exist, both return
// 404, to avoid revealing it exists.
func UpdateECPart(repo repository.ECPartRepository, er repoapi.ECPart) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("ecPartId")

		if !authz.IsOwner(r.Context(), r.PathValue("userId")) {
			problem.NotFound(w, "resource not found")
			return
		}

		p, ok := decodeECPartInput(w, r)
		if !ok {
			return
		}
		p.ID = id

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.ECPartID, id)
			problem.Internal(w, "failed to update EC part")
			return
		}

		updated, err := repo.Update(r.Context(), p)
		if handleMutationError(w, r, err, log.ECPartID, id) {
			return
		}

		out, err := er.ToAPI(*updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping EC part to API", log.Error, err, log.ECPartID, id)
			problem.Internal(w, "failed to update EC part")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}
