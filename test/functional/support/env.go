package support

import (
	"os"
	"path/filepath"
	"runtime"
)

// BaseURL returns the API's base URL under test, e.g. "http://127.0.0.1:3000"
// for a local sam local start-api instance or the real deployed mykeebs-api-dev API
// Gateway URL. Configurable via MYKEEBS_API_BASE_URL.
func BaseURL() string {
	if v := os.Getenv("MYKEEBS_API_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:3000"
}

// OidcIssuer is the iss claim every minted test token must carry, matching the
// deployed authorizer's configured issuer.
func OidcIssuer() string {
	return os.Getenv("MYKEEBS_OIDC_ISSUER")
}

// OidcAudience is the aud claim every minted test token must carry.
func OidcAudience() string {
	return os.Getenv("MYKEEBS_OIDC_AUDIENCE")
}

// OidcSigningKeyPath is the PEM oidc-testkit-gen wrote pre-deploy; the suite
// loads it once and signs every token against it.
func OidcSigningKeyPath() string {
	return os.Getenv("MYKEEBS_OIDC_SIGNING_KEY_PATH")
}

// SwitchTableName returns the DynamoDB table name backing
// GET /users/{userId}/switches, so specs can seed fixture rows directly.
func SwitchTableName() string {
	return os.Getenv("MYKEEBS_SWITCH_TABLE_NAME")
}

// ECPartTableName returns the DynamoDB table name backing
// GET /users/{userId}/ec-parts, so specs can seed fixture rows directly.
func ECPartTableName() string {
	return os.Getenv("MYKEEBS_EC_PART_TABLE_NAME")
}

// KeyboardTableName returns the DynamoDB table name backing
// GET /users/{userId}/keyboards, so specs can seed fixture rows directly.
func KeyboardTableName() string {
	return os.Getenv("MYKEEBS_KEYBOARD_TABLE_NAME")
}

// KeycapSetTableName returns the DynamoDB table name backing
// GET /users/{userId}/keycap-sets, so specs can seed fixture rows directly.
func KeycapSetTableName() string {
	return os.Getenv("MYKEEBS_KEYCAP_SET_TABLE_NAME")
}

// BuildTableName returns the DynamoDB table name backing
// POST /users/{userId}/builds, so specs can clean up fixture rows directly.
func BuildTableName() string {
	return os.Getenv("MYKEEBS_BUILD_TABLE_NAME")
}

// ProfileTableName returns the DynamoDB table name backing
// GET /v1/profile/{identifier}, so specs can seed fixture rows directly.
func ProfileTableName() string {
	return os.Getenv("MYKEEBS_PROFILE_TABLE_NAME")
}

// ProfileUsernameTableName returns the DynamoDB table name holding
// { username -> user_id } claim items, so specs can seed the claim needed
// to resolve a profile by username.
func ProfileUsernameTableName() string {
	return os.Getenv("MYKEEBS_PROFILE_USERNAME_TABLE_NAME")
}

// UploadCounterTableName returns the DynamoDB table name holding the
// per-day image upload counts, so specs can put a day at its limit.
func UploadCounterTableName() string {
	return os.Getenv("MYKEEBS_UPLOAD_COUNTER_TABLE_NAME")
}

// ImagesBucketName returns the bucket uploaded images are stored in.
func ImagesBucketName() string {
	return os.Getenv("MYKEEBS_IMAGES_BUCKET_NAME")
}

// AlarmTopicArn returns the stack's alarm topic, which also carries
// account delete notices.
func AlarmTopicArn() string {
	return os.Getenv("MYKEEBS_ALARM_TOPIC_ARN")
}

// StackName returns the deployed stack's name, which prefixes any AWS
// resource a spec creates so the CI role may manage it.
func StackName() string {
	return os.Getenv("MYKEEBS_STACK_NAME")
}

// AWSEndpointURL returns the endpoint for the other AWS services specs
// call directly (SNS, SQS). Empty uses default AWS endpoint resolution.
func AWSEndpointURL() string {
	return os.Getenv("MYKEEBS_AWS_ENDPOINT_URL")
}

// S3EndpointURL returns the S3 endpoint specs should talk to. Empty uses
// default AWS endpoint resolution.
func S3EndpointURL() string {
	return os.Getenv("MYKEEBS_S3_ENDPOINT_URL")
}

// DynamoDBEndpointURL returns the DynamoDB endpoint specs should talk to.
// Empty uses default AWS endpoint resolution.
func DynamoDBEndpointURL() string {
	return os.Getenv("MYKEEBS_DYNAMODB_ENDPOINT_URL")
}

// CACertPath returns a PEM CA certificate to trust on top of the system
// roots when fetching image URLs: floci's test CA locally, empty against real
// AWS.
func CACertPath() string {
	return os.Getenv("MYKEEBS_CA_CERT_PATH")
}

// ImagesDomain returns the host image URLs must be served from: the stack's
// ImagesDomain output.
func ImagesDomain() string {
	return os.Getenv("MYKEEBS_IMAGES_DOMAIN")
}

// IsCI reports whether specs are running in CI, which targets real AWS
// rather than the local emulator.
func IsCI() bool {
	return os.Getenv("CI") == "true"
}

// ImagesDistributionID returns the stack's images distribution.
func ImagesDistributionID() string {
	return os.Getenv("MYKEEBS_IMAGES_DISTRIBUTION_ID")
}

// ImagesInvalidation reports whether the stack invalidates deleted images:
// off in CI's PR stacks, on for floci.
func ImagesInvalidation() bool {
	return os.Getenv("MYKEEBS_IMAGES_INVALIDATION") == "true"
}

// RepoRoot returns the mykeebs-api repository's root directory.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}
