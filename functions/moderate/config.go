package main

// Config is populated from environment variables via Kong, like
// functions/api's.
type Config struct {
	SwitchTableName          string `env:"SWITCH_TABLE_NAME" required:""`
	ECPartTableName          string `env:"EC_PART_TABLE_NAME" required:""`
	KeyboardTableName        string `env:"KEYBOARD_TABLE_NAME" required:""`
	KeycapSetTableName       string `env:"KEYCAP_SET_TABLE_NAME" required:""`
	BuildTableName           string `env:"BUILD_TABLE_NAME" required:""`
	ProfileTableName         string `env:"PROFILE_TABLE_NAME" required:""`
	ProfileUsernameTableName string `env:"PROFILE_USERNAME_TABLE_NAME" required:""`

	// Logs every label Rekognition sees, for tuning the thresholds - see
	// moderation.New.
	Calibration bool `env:"MODERATION_CALIBRATION"`
}
