package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/cascadedelete"
	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/lookup"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/schema"
	"github.com/roguekeyboards/mykeebs-api/internal/repomcp"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
)

var createKeyboardPlateTool = &mcp.Tool{
	Name:        "create_keyboard_plate",
	Description: "Adds a plate to a keyboard in your own collection, after its existing plates. Give it a purchase only if it was bought or priced separately; the keyboard's own purchase price then excludes it. material and purchase.vendor/order_status must be approved lookup values - call list_lookups and get_lookup to see them.",
}

var updateKeyboardPlateTool = &mcp.Tool{
	Name:        "update_keyboard_plate",
	Description: "Replaces a plate on a keyboard in your own collection. Every field is replaced, so omitting an optional field clears it; send the full plate. The plate keeps its id, so builds using it still do.",
}

var deleteKeyboardPlateTool = &mcp.Tool{
	Name:        "delete_keyboard_plate",
	Description: "Removes a plate from a keyboard in your own collection. Idempotent: deleting a plate that isn't there succeeds. on_delete controls what happens if a build uses the plate: \"block\" (default) fails and lists those build ids; \"cascade\" deletes the plate and every build using it.",
}

var createKeyboardPCBTool = &mcp.Tool{
	Name:        "create_keyboard_pcb",
	Description: "Adds a PCB to a keyboard in your own collection, after its existing PCBs. Give it a purchase only if it was bought or priced separately; the keyboard's own purchase price then excludes it. firmware, assembly, connectivity and purchase.vendor/order_status must be approved lookup values - call list_lookups and get_lookup to see them.",
}

var updateKeyboardPCBTool = &mcp.Tool{
	Name:        "update_keyboard_pcb",
	Description: "Replaces a PCB on a keyboard in your own collection. Every field is replaced, so omitting an optional field clears it; send the full PCB. The PCB keeps its id, so builds using it still do.",
}

var deleteKeyboardPCBTool = &mcp.Tool{
	Name:        "delete_keyboard_pcb",
	Description: "Removes a PCB from a keyboard in your own collection. Idempotent: deleting a PCB that isn't there succeeds. on_delete controls what happens if a build uses the PCB: \"block\" (default) fails and lists those build ids; \"cascade\" deletes the PCB and every build using it.",
}

func handleCreateKeyboardPlate(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateKeyboardPlateInput, schema.CreateKeyboardPlateOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeyboardPlateInput) (*mcp.CallToolResult, schema.CreateKeyboardPlateOutput, error) {
		plate, err := validatedKeyboardPlate(ctx, in.KeyboardPlateInput)
		if err != nil {
			return nil, schema.CreateKeyboardPlateOutput{}, err
		}

		plate.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, in.KeyboardID, log.KeyboardPlate, plate.ID)
			return nil, schema.CreateKeyboardPlateOutput{}, errors.New("failed to create keyboard plate")
		}

		created, err := keyboardRepo.AddPlate(ctx, in.KeyboardID, plate)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID, log.KeyboardPlate, plate.ID); mutErr != nil {
			return nil, schema.CreateKeyboardPlateOutput{}, mutErr
		}

		// isOwner: true - a plate is always added to the caller's own keyboard.
		return nil, schema.CreateKeyboardPlateOutput{Plate: repomcp.Keyboard{}.PlateToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateKeyboardPlate(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateKeyboardPlateInput, schema.UpdateKeyboardPlateOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeyboardPlateInput) (*mcp.CallToolResult, schema.UpdateKeyboardPlateOutput, error) {
		plate, err := validatedKeyboardPlate(ctx, in.KeyboardPlateInput)
		if err != nil {
			return nil, schema.UpdateKeyboardPlateOutput{}, err
		}

		plate.ID = in.PlateID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, in.KeyboardID, log.KeyboardPlate, in.PlateID)
			return nil, schema.UpdateKeyboardPlateOutput{}, errors.New("failed to update keyboard plate")
		}

		updated, err := keyboardRepo.UpdatePlate(ctx, in.KeyboardID, plate)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID, log.KeyboardPlate, in.PlateID); mutErr != nil {
			return nil, schema.UpdateKeyboardPlateOutput{}, mutErr
		}

		// isOwner: true - this always targets the caller's own keyboard.
		return nil, schema.UpdateKeyboardPlateOutput{Plate: repomcp.Keyboard{}.PlateToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteKeyboardPlate(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardPlateInput, schema.DeleteKeyboardPlateOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardPlateInput) (*mcp.CallToolResult, schema.DeleteKeyboardPlateOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeyboardPlateOutput{}, err
		}

		result, err := cascadedelete.DeleteKeyboardPlate(ctx, keyboardRepo, buildRepo, buildImages, ownerID, in.KeyboardID, in.PlateID, onDelete)
		if mutErr := partDeleteError(ctx, err, "plate", log.KeyboardID, in.KeyboardID, log.KeyboardPlate, in.PlateID); mutErr != nil {
			return nil, schema.DeleteKeyboardPlateOutput{}, mutErr
		}

		return nil, schema.DeleteKeyboardPlateOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func handleCreateKeyboardPCB(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.CreateKeyboardPCBInput, schema.CreateKeyboardPCBOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeyboardPCBInput) (*mcp.CallToolResult, schema.CreateKeyboardPCBOutput, error) {
		pcb, err := validatedKeyboardPCB(ctx, in.KeyboardPCBInput)
		if err != nil {
			return nil, schema.CreateKeyboardPCBOutput{}, err
		}

		pcb.ID = uuid.NewString()

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, in.KeyboardID, log.KeyboardPCB, pcb.ID)
			return nil, schema.CreateKeyboardPCBOutput{}, errors.New("failed to create keyboard PCB")
		}

		created, err := keyboardRepo.AddPCB(ctx, in.KeyboardID, pcb)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID, log.KeyboardPCB, pcb.ID); mutErr != nil {
			return nil, schema.CreateKeyboardPCBOutput{}, mutErr
		}

		// isOwner: true - a PCB is always added to the caller's own keyboard.
		return nil, schema.CreateKeyboardPCBOutput{PCB: repomcp.Keyboard{}.PCBToMCP(*created, true, ownerPrefs)}, nil
	}
}

func handleUpdateKeyboardPCB(
	keyboardRepo repository.KeyboardRepository,
	prefs repository.PreferencesReader,
) mcp.ToolHandlerFor[schema.UpdateKeyboardPCBInput, schema.UpdateKeyboardPCBOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeyboardPCBInput) (*mcp.CallToolResult, schema.UpdateKeyboardPCBOutput, error) {
		pcb, err := validatedKeyboardPCB(ctx, in.KeyboardPCBInput)
		if err != nil {
			return nil, schema.UpdateKeyboardPCBOutput{}, err
		}

		pcb.ID = in.PCBID

		ownerPrefs, err := callerPreferences(ctx, prefs)
		if err != nil {
			log.FromContext(ctx).Error("getting owner preferences", log.Error, err, log.KeyboardID, in.KeyboardID, log.KeyboardPCB, in.PCBID)
			return nil, schema.UpdateKeyboardPCBOutput{}, errors.New("failed to update keyboard PCB")
		}

		updated, err := keyboardRepo.UpdatePCB(ctx, in.KeyboardID, pcb)
		if mutErr := handleMutationError(ctx, err, log.KeyboardID, in.KeyboardID, log.KeyboardPCB, in.PCBID); mutErr != nil {
			return nil, schema.UpdateKeyboardPCBOutput{}, mutErr
		}

		// isOwner: true - this always targets the caller's own keyboard.
		return nil, schema.UpdateKeyboardPCBOutput{PCB: repomcp.Keyboard{}.PCBToMCP(*updated, true, ownerPrefs)}, nil
	}
}

func handleDeleteKeyboardPCB(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardPCBInput, schema.DeleteKeyboardPCBOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardPCBInput) (*mcp.CallToolResult, schema.DeleteKeyboardPCBOutput, error) {
		onDelete, _ := cascadedelete.ParseOnDelete(in.OnDelete)

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeyboardPCBOutput{}, err
		}

		result, err := cascadedelete.DeleteKeyboardPCB(ctx, keyboardRepo, buildRepo, buildImages, ownerID, in.KeyboardID, in.PCBID, onDelete)
		if mutErr := partDeleteError(ctx, err, "PCB", log.KeyboardID, in.KeyboardID, log.KeyboardPCB, in.PCBID); mutErr != nil {
			return nil, schema.DeleteKeyboardPCBOutput{}, mutErr
		}

		return nil, schema.DeleteKeyboardPCBOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

// partDeleteError maps a part delete's error to the tool's error, listing
// the blocking builds when on_delete=block stopped it.
func partDeleteError(ctx context.Context, err error, kind string, logFields ...any) error {
	if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
		return fmt.Errorf("keyboard %s is still used by builds: %s", kind, strings.Join(blocked.BuildIDs, ", "))
	}
	return handleMutationError(ctx, err, logFields...)
}

func validatedKeyboardPlate(ctx context.Context, in schema.KeyboardPlateInput) (repository.KeyboardPlate, error) {
	plate := repomcp.Keyboard{}.PlateFromMCP(in)
	if err := lookupErrors(lookup.ValidateKeyboardPlate(ctx, plate)); err != nil {
		return repository.KeyboardPlate{}, err
	}

	return plate, nil
}

func validatedKeyboardPCB(ctx context.Context, in schema.KeyboardPCBInput) (repository.KeyboardPCB, error) {
	pcb := repomcp.Keyboard{}.PCBFromMCP(in)
	if err := lookupErrors(lookup.ValidateKeyboardPCB(ctx, pcb)); err != nil {
		return repository.KeyboardPCB{}, err
	}

	return pcb, nil
}
