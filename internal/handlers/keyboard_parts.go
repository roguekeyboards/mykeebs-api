package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

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
)

// writePart writes part as JSON with status.
func writePart(w http.ResponseWriter, status int, part any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(part)
}

// CreateKeyboardPlate reads the {userId} and {keyboardId} path values and
// requires an authenticated caller. A plate has no visibility of its own -
// authorization is entirely the keyboard's ownership. userId must be the
// caller's own subject; adding a plate to another user's keyboard, or to
// one that doesn't exist, both return 404.
func CreateKeyboardPlate(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeyboardPlateInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		plate := kr.PlateToRepo(in)
		if !writeLookupErrors(w, lookup.ValidateKeyboardPlate(r.Context(), plate)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(plate.Purchase)) {
			return
		}

		plate.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPlate, plate.ID)
			problem.Internal(w, "failed to add plate")
			return
		}

		created, err := keyboardRepo.AddPlate(r.Context(), keyboardID, plate)
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID, log.KeyboardPlate, plate.ID) {
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := kr.PlateToAPI(*created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard plate to API", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPlate, created.ID)
			problem.Internal(w, "failed to add plate")
			return
		}

		writePart(w, http.StatusCreated, out)
	}
}

// UpdateKeyboardPlate reads the {userId}, {keyboardId} and {plateId} path
// values and requires an authenticated caller. userId must be the caller's
// own subject; updating a plate on another user's keyboard, a keyboard
// that doesn't exist, or a plateId it doesn't have, all return 404.
func UpdateKeyboardPlate(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")
		plateID := r.PathValue("plateId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeyboardPlateInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		plate := kr.PlateToRepo(in)
		if !writeLookupErrors(w, lookup.ValidateKeyboardPlate(r.Context(), plate)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(plate.Purchase)) {
			return
		}

		plate.ID = plateID

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPlate, plateID)
			problem.Internal(w, "failed to update plate")
			return
		}

		updated, err := keyboardRepo.UpdatePlate(r.Context(), keyboardID, plate)
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID, log.KeyboardPlate, plateID) {
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := kr.PlateToAPI(*updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard plate to API", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPlate, plateID)
			problem.Internal(w, "failed to update plate")
			return
		}

		writePart(w, http.StatusOK, out)
	}
}

// DeleteKeyboardPlate reads the {userId}, {keyboardId} and {plateId} path
// values and requires an authenticated caller. userId must be the caller's
// own subject; deleting a plate from another user's keyboard, or a
// keyboard that doesn't exist, both return 404. Idempotent: a plateId the
// keyboard doesn't have is not an error. The on_delete query param
// (default "block") controls what happens if a build uses the plate: see
// [cascadedelete.DeleteKeyboardPlate].
func DeleteKeyboardPlate(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")
		plateID := r.PathValue("plateId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be block or cascade")
			return
		}

		result, err := cascadedelete.DeleteKeyboardPlate(r.Context(), keyboardRepo, buildRepo, buildImages, ownerID, keyboardID, plateID, onDelete)
		writePartDeleteResult(w, r, result, err, onDelete, "plate", log.KeyboardID, keyboardID, log.KeyboardPlate, plateID)
	}
}

// CreateKeyboardPCB is CreateKeyboardPlate for PCBs.
func CreateKeyboardPCB(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeyboardPCBInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		pcb := kr.PCBToRepo(in)
		if !writeLookupErrors(w, lookup.ValidateKeyboardPCB(r.Context(), pcb)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(pcb.Purchase)) {
			return
		}

		pcb.ID = uuid.NewString()

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPCB, pcb.ID)
			problem.Internal(w, "failed to add PCB")
			return
		}

		created, err := keyboardRepo.AddPCB(r.Context(), keyboardID, pcb)
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID, log.KeyboardPCB, pcb.ID) {
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := kr.PCBToAPI(*created, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard PCB to API", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPCB, created.ID)
			problem.Internal(w, "failed to add PCB")
			return
		}

		writePart(w, http.StatusCreated, out)
	}
}

// UpdateKeyboardPCB is UpdateKeyboardPlate for PCBs.
func UpdateKeyboardPCB(keyboardRepo repository.KeyboardRepository, kr repoapi.Keyboard) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")
		pcbID := r.PathValue("pcbId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeyboardPCBInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		pcb := kr.PCBToRepo(in)
		if !writeLookupErrors(w, lookup.ValidateKeyboardPCB(r.Context(), pcb)) {
			return
		}
		if !writePurchaseErrors(w, purchasevalidate.KeyboardPurchase(pcb.Purchase)) {
			return
		}

		pcb.ID = pcbID

		ownerPrefs, err := ownerprefs.Get(r.Context())
		if err != nil {
			log.FromContext(r.Context()).Error("getting owner preferences", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPCB, pcbID)
			problem.Internal(w, "failed to update PCB")
			return
		}

		updated, err := keyboardRepo.UpdatePCB(r.Context(), keyboardID, pcb)
		if handleMutationError(w, r, err, log.KeyboardID, keyboardID, log.KeyboardPCB, pcbID) {
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := kr.PCBToAPI(*updated, true, ownerPrefs)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard PCB to API", log.Error, err, log.KeyboardID, keyboardID, log.KeyboardPCB, pcbID)
			problem.Internal(w, "failed to update PCB")
			return
		}

		writePart(w, http.StatusOK, out)
	}
}

// DeleteKeyboardPCB is DeleteKeyboardPlate for PCBs.
func DeleteKeyboardPCB(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		keyboardID := r.PathValue("keyboardId")
		pcbID := r.PathValue("pcbId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be block or cascade")
			return
		}

		result, err := cascadedelete.DeleteKeyboardPCB(r.Context(), keyboardRepo, buildRepo, buildImages, ownerID, keyboardID, pcbID, onDelete)
		writePartDeleteResult(w, r, result, err, onDelete, "PCB", log.KeyboardID, keyboardID, log.KeyboardPCB, pcbID)
	}
}

// writePartDeleteResult maps a part delete's outcome to a response: 409 if
// blocked, 200 with the deleted build ids on cascade, 204 otherwise.
func writePartDeleteResult(
	w http.ResponseWriter, r *http.Request, result cascadedelete.Result, err error,
	onDelete cascadedelete.OnDelete, kind string, logFields ...any,
) {
	if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
		problem.StillReferenced(w, kind+" is still used by one or more builds", blocked.BuildIDs)
		return
	}
	if handleMutationError(w, r, err, logFields...) {
		return
	}

	if onDelete == cascadedelete.OnDeleteCascade {
		writePart(w, http.StatusOK, api.CascadeDeleteResult{DeletedBuildIds: result.DeletedBuildIDs})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
