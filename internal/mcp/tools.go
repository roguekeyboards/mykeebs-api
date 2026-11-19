package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
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
	sdkmcp.AddTool(s, listLookupsTool, handleListLookups())
	sdkmcp.AddTool(s, getLookupTool, handleGetLookup())
	sdkmcp.AddTool(s, getProfileTool, handleGetProfile(profileRepo))
	sdkmcp.AddTool(s, createProfileTool, handleCreateProfile(profileRepo))
	sdkmcp.AddTool(s, updateProfileTool, handleUpdateProfile(profileRepo))
	sdkmcp.AddTool(s, deleteProfileTool, handleDeleteProfile(account))
	sdkmcp.AddTool(s, listProfilesTool, handleListProfiles(profileRepo))
	sdkmcp.AddTool(s, setProfileImageTool, handleSetProfileImage(profileRepo, profileImageStore, uploads))
	sdkmcp.AddTool(s, deleteProfileImageTool, handleDeleteProfileImage(profileRepo, profileImageStore))
	sdkmcp.AddTool(s, listSwitchesTool, handleListSwitches(switchRepo, profileRepo))
	sdkmcp.AddTool(s, getSwitchTool, handleGetSwitch(switchRepo, profileRepo))
	sdkmcp.AddTool(s, createSwitchTool, handleCreateSwitch(switchRepo, profileRepo))
	sdkmcp.AddTool(s, updateSwitchTool, handleUpdateSwitch(switchRepo, profileRepo))
	sdkmcp.AddTool(s, deleteSwitchTool, handleDeleteSwitch(switchRepo, buildRepo, buildImageStore, switchImageStore))
	sdkmcp.AddTool(s, setSwitchImageTool, handleSetSwitchImage(switchRepo, switchImageStore, uploads))
	sdkmcp.AddTool(s, deleteSwitchImageTool, handleDeleteSwitchImage(switchRepo, switchImageStore))
	sdkmcp.AddTool(s, listECPartsTool, handleListECParts(ecPartRepo, profileRepo))
	sdkmcp.AddTool(s, getECPartTool, handleGetECPart(ecPartRepo, profileRepo))
	sdkmcp.AddTool(s, createECPartTool, handleCreateECPart(ecPartRepo, profileRepo))
	sdkmcp.AddTool(s, updateECPartTool, handleUpdateECPart(ecPartRepo, profileRepo))
	sdkmcp.AddTool(s, deleteECPartTool, handleDeleteECPart(ecPartRepo, buildRepo, buildImageStore, ecPartImageStore))
	sdkmcp.AddTool(s, setECPartImageTool, handleSetECPartImage(ecPartRepo, ecPartImageStore, uploads))
	sdkmcp.AddTool(s, deleteECPartImageTool, handleDeleteECPartImage(ecPartRepo, ecPartImageStore))
	sdkmcp.AddTool(s, listKeyboardsTool, handleListKeyboards(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, getKeyboardTool, handleGetKeyboard(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, listKeyboardImagesTool, handleListKeyboardImages(keyboardRepo))
	sdkmcp.AddTool(s, createKeyboardTool, handleCreateKeyboard(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, updateKeyboardTool, handleUpdateKeyboard(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, deleteKeyboardTool, handleDeleteKeyboard(keyboardRepo, buildRepo, buildImageStore, keyboardImageStore))
	sdkmcp.AddTool(s, createKeyboardPlateTool, handleCreateKeyboardPlate(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, updateKeyboardPlateTool, handleUpdateKeyboardPlate(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, deleteKeyboardPlateTool, handleDeleteKeyboardPlate(keyboardRepo, buildRepo, buildImageStore))
	sdkmcp.AddTool(s, createKeyboardPCBTool, handleCreateKeyboardPCB(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, updateKeyboardPCBTool, handleUpdateKeyboardPCB(keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, deleteKeyboardPCBTool, handleDeleteKeyboardPCB(keyboardRepo, buildRepo, buildImageStore))
	sdkmcp.AddTool(s, addKeyboardImageTool, handleAddKeyboardImage(keyboardRepo, keyboardImageStore, uploads))
	sdkmcp.AddTool(s, deleteKeyboardImageTool, handleDeleteKeyboardImage(keyboardRepo, keyboardImageStore))
	sdkmcp.AddTool(s, listKeycapSetsTool, handleListKeycapSets(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, getKeycapSetTool, handleGetKeycapSet(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, createKeycapSetTool, handleCreateKeycapSet(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, updateKeycapSetTool, handleUpdateKeycapSet(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, deleteKeycapSetTool, handleDeleteKeycapSet(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	sdkmcp.AddTool(s, createKeycapKitTool, handleCreateKeycapKit(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, updateKeycapKitTool, handleUpdateKeycapKit(keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, deleteKeycapKitTool, handleDeleteKeycapKit(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	sdkmcp.AddTool(s, setKeycapKitImageTool, handleSetKeycapKitImage(keycapSetRepo, imageStore, uploads))
	sdkmcp.AddTool(s, deleteKeycapKitImageTool, handleDeleteKeycapKitImage(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createBuildTool, handleCreateBuild(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, getBuildTool, handleGetBuild(buildRepo, keyboardRepo, profileRepo))
	sdkmcp.AddTool(s, listBuildsTool, handleListBuilds(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, updateBuildTool, handleUpdateBuild(buildRepo, keyboardRepo, switchRepo, ecPartRepo, keycapSetRepo, profileRepo))
	sdkmcp.AddTool(s, deleteBuildTool, handleDeleteBuild(buildRepo, buildImageStore))
	sdkmcp.AddTool(s, addBuildImageTool, handleAddBuildImage(buildRepo, buildImageStore, uploads))
	sdkmcp.AddTool(s, deleteBuildImageTool, handleDeleteBuildImage(buildRepo, buildImageStore))
	sdkmcp.AddTool(s, listBuildImagesTool, handleListBuildImages(buildRepo))
}
