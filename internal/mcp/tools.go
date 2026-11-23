package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/mcp/validate"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// registerTools is the single list of every tool this server exposes -
// add a new tool here, not by editing New.
func registerTools(
	s *sdkmcp.Server,
	switchRepo repository.SwitchRepository,
	switchImageStore repository.SwitchImageStore,
	ecPartRepo repository.ECPartRepository,
	ecPartImageStore repository.ECPartImageStore,
	keyboardRepo repository.KeyboardRepository,
	keyboardImageStore repository.KeyboardImageStore,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
	buildImageStore repository.BuildImageStore,
	profileRepo repository.ProfileRepository,
	profileImageStore repository.ProfileImageStore,
	account accountdelete.Account,
	uploads *uploadlimit.Limiter,
) {
	addTool(s, listLookupsTool, handleListLookups())
	addTool(s, getLookupTool, handleGetLookup())
	addTool(s, getProfileTool, handleGetProfile(profileRepo))
	addTool(s, createProfileTool, handleCreateProfile(profileRepo))
	addTool(s, updateProfileTool, handleUpdateProfile(profileRepo))
	addTool(s, deleteProfileTool, handleDeleteProfile(account))
	addTool(s, listProfilesTool, handleListProfiles(profileRepo))
	addTool(s, setProfileImageTool, handleSetProfileImage(profileRepo, profileImageStore, uploads))
	addTool(s, deleteProfileImageTool, handleDeleteProfileImage(profileRepo, profileImageStore))
	addTool(s, listSwitchesTool, handleListSwitches(switchRepo, profileRepo))
	addTool(s, getSwitchTool, handleGetSwitch(switchRepo, profileRepo))
	addTool(s, createSwitchTool, handleCreateSwitch(switchRepo, profileRepo))
	addTool(s, updateSwitchTool, handleUpdateSwitch(switchRepo, profileRepo))
	addTool(s, deleteSwitchTool, handleDeleteSwitch(switchRepo, buildRepo, buildImageStore, switchImageStore))
	addTool(s, setSwitchImageTool, handleSetSwitchImage(switchRepo, switchImageStore, uploads))
	addTool(s, deleteSwitchImageTool, handleDeleteSwitchImage(switchRepo, switchImageStore))
	addTool(s, listECPartsTool, handleListECParts(ecPartRepo, profileRepo))
	addTool(s, getECPartTool, handleGetECPart(ecPartRepo, profileRepo))
	addTool(s, createECPartTool, handleCreateECPart(ecPartRepo, profileRepo))
	addTool(s, updateECPartTool, handleUpdateECPart(ecPartRepo, profileRepo))
	addTool(s, deleteECPartTool, handleDeleteECPart(ecPartRepo, buildRepo, buildImageStore, ecPartImageStore))
	addTool(s, setECPartImageTool, handleSetECPartImage(ecPartRepo, ecPartImageStore, uploads))
	addTool(s, deleteECPartImageTool, handleDeleteECPartImage(ecPartRepo, ecPartImageStore))
	addTool(s, listKeyboardsTool, handleListKeyboards(keyboardRepo, profileRepo))
	addTool(s, getKeyboardTool, handleGetKeyboard(keyboardRepo, profileRepo))
	addTool(s, listKeyboardImagesTool, handleListKeyboardImages(keyboardRepo))
	addTool(s, createKeyboardTool, handleCreateKeyboard(keyboardRepo, profileRepo))
	addTool(s, updateKeyboardTool, handleUpdateKeyboard(keyboardRepo, profileRepo))
	addTool(s, deleteKeyboardTool, handleDeleteKeyboard(keyboardRepo, buildRepo, buildImageStore, keyboardImageStore))
	addTool(s, createKeyboardPlateTool, handleCreateKeyboardPlate(keyboardRepo, profileRepo))
	addTool(s, updateKeyboardPlateTool, handleUpdateKeyboardPlate(keyboardRepo, profileRepo))
	addTool(s, deleteKeyboardPlateTool, handleDeleteKeyboardPlate(keyboardRepo, buildRepo, buildImageStore))
	addTool(s, createKeyboardPCBTool, handleCreateKeyboardPCB(keyboardRepo, profileRepo))
	addTool(s, updateKeyboardPCBTool, handleUpdateKeyboardPCB(keyboardRepo, profileRepo))
	addTool(s, deleteKeyboardPCBTool, handleDeleteKeyboardPCB(keyboardRepo, buildRepo, buildImageStore))
	addTool(s, addKeyboardImageTool, handleAddKeyboardImage(keyboardRepo, keyboardImageStore, uploads))
	addTool(s, deleteKeyboardImageTool, handleDeleteKeyboardImage(keyboardRepo, keyboardImageStore))
	addTool(s, listKeycapSetsTool, handleListKeycapSets(keycapSetRepo, profileRepo))
	addTool(s, getKeycapSetTool, handleGetKeycapSet(keycapSetRepo, profileRepo))
	addTool(s, createKeycapSetTool, handleCreateKeycapSet(keycapSetRepo, profileRepo))
	addTool(s, updateKeycapSetTool, handleUpdateKeycapSet(keycapSetRepo, profileRepo))
	addTool(s, deleteKeycapSetTool, handleDeleteKeycapSet(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	addTool(s, createKeycapKitTool, handleCreateKeycapKit(keycapSetRepo, profileRepo))
	addTool(s, updateKeycapKitTool, handleUpdateKeycapKit(keycapSetRepo, profileRepo))
	addTool(s, deleteKeycapKitTool, handleDeleteKeycapKit(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	addTool(s, setKeycapKitImageTool, handleSetKeycapKitImage(keycapSetRepo, imageStore, uploads))
	addTool(s, deleteKeycapKitImageTool, handleDeleteKeycapKitImage(keycapSetRepo, imageStore))
	addTool(s, createBuildTool, handleCreateBuild(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	addTool(s, getBuildTool, handleGetBuild(buildRepo, keyboardRepo, profileRepo))
	addTool(s, listBuildsTool, handleListBuilds(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	addTool(s, updateBuildTool, handleUpdateBuild(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	addTool(s, deleteBuildTool, handleDeleteBuild(buildRepo, buildImageStore))
	addTool(s, addBuildImageTool, handleAddBuildImage(buildRepo, buildImageStore, uploads))
	addTool(s, deleteBuildImageTool, handleDeleteBuildImage(buildRepo, buildImageStore))
	addTool(s, listBuildImagesTool, handleListBuildImages(buildRepo))
}

// addTool rejects input that breaks its type's [validate.Validatable]
// rules before h runs.
func addTool[In, Out any](s *sdkmcp.Server, t *sdkmcp.Tool, h sdkmcp.ToolHandlerFor[In, Out]) {
	sdkmcp.AddTool(s, t, func(ctx context.Context, req *sdkmcp.CallToolRequest, in In) (*sdkmcp.CallToolResult, Out, error) {
		if x, ok := any(in).(validate.Validatable); ok {
			if err := validate.Error(x); err != nil {
				var zero Out
				return nil, zero, err
			}
		}
		return h(ctx, req, in)
	})
}
