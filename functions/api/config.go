package main

// Config is populated from environment variables via Kong. There are no CLI
// flags for this service — it's a Lambda entrypoint, not a CLI tool — but
// Kong's struct-tag env binding, defaults, and required-field validation are
// useful regardless of whether flag parsing is ever exercised.
type Config struct {
	OIDCIssuerURL string `env:"OIDC_ISSUER_URL" required:""`
	OIDCAudience  string `env:"OIDC_AUDIENCE" required:""`
	// The IdP's browser-SDK public token, rendered into the GET /authorize
	// consent page - see internal/consent.
	IDPConsentPublicToken string `env:"IDP_CONSENT_PUBLIC_TOKEN" required:""`
	ImagesBucketName      string `env:"IMAGES_BUCKET_NAME" required:""`
	// Image GET URLs are CloudFront signed URLs under this base, e.g.
	// https://images.mykeebs.info.
	ImagesBaseURL string `env:"IMAGES_BASE_URL" required:""`
	// SSM parameters holding the image signing key pair's private half and
	// its CloudFront public key ID. Both are read at cold start in one call,
	// so a key rotation can't pair a new key with an old ID.
	ImagesSigningKeyParam   string `env:"IMAGES_SIGNING_KEY_PARAM" required:""`
	ImagesSigningKeyIDParam string `env:"IMAGES_SIGNING_KEY_ID_PARAM" required:""`
	// The images distribution whose edge copies are invalidated when an
	// image is deleted. Empty invalidates nothing.
	ImagesDistributionID     string `env:"IMAGES_DISTRIBUTION_ID"`
	SwitchTableName          string `env:"SWITCH_TABLE_NAME" required:""`
	ECPartTableName          string `env:"EC_PART_TABLE_NAME" required:""`
	KeyboardTableName        string `env:"KEYBOARD_TABLE_NAME" required:""`
	KeycapSetTableName       string `env:"KEYCAP_SET_TABLE_NAME" required:""`
	BuildTableName           string `env:"BUILD_TABLE_NAME" required:""`
	ProfileTableName         string `env:"PROFILE_TABLE_NAME" required:""`
	ProfileUsernameTableName string `env:"PROFILE_USERNAME_TABLE_NAME" required:""`
	UploadCounterTableName   string `env:"UPLOAD_COUNTER_TABLE_NAME" required:""`
	// Upload URLs issued per UTC day across every user - see
	// internal/uploadlimit.
	DailyUploadLimit int64 `env:"DAILY_UPLOAD_LIMIT" required:""`

	// SNS topic the operator gets account delete notices on - see
	// internal/deletenotice. Empty sends none.
	AccountDeleteTopicARN string `env:"ACCOUNT_DELETE_TOPIC_ARN"`

	// Empty in real deployments; set locally to point at an emulator.
	DynamoDBEndpointURL string `env:"DYNAMODB_ENDPOINT_URL"`

	// Empty in real deployments; set locally to point at an emulator.
	S3EndpointURL string `env:"S3_ENDPOINT_URL"`
}
