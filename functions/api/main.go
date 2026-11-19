// Command api is the mykeebs-api API server. It is a plain net/http server with no
// Lambda-specific code — in production it runs behind the aws-lambda-web-adapter
// extension, which translates API Gateway events into real HTTP requests
// against this process.
package main

import (
	"context"
	"crypto"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
	awscloudfront "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/roguekeyboards/mykeebs-api/internal/accountdelete"
	"github.com/roguekeyboards/mykeebs-api/internal/auth"
	"github.com/roguekeyboards/mykeebs-api/internal/deletenotice"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/cloudfront"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/dynamo"
	imagestore "github.com/roguekeyboards/mykeebs-api/internal/repository/s3"
	"github.com/roguekeyboards/mykeebs-api/internal/router"
	"github.com/roguekeyboards/mykeebs-api/internal/uploadlimit"
)

// Version is set at build time via -ldflags "-X main.Version=...", derived
// from `git describe --tags --always --dirty` (see functions/api/Dockerfile).
// Defaults to "dev" when built without that flag (e.g. `go build` directly).
var Version = "dev"

// port must match AWS_LWA_PORT in template.yaml; not 8080 since sam local
// start-api's Runtime Interface Emulator already binds that port in-container
// (github.com/aws/aws-lambda-web-adapter#125).
const port = "8000"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var cfg Config
	kong.Parse(&cfg)

	ctx := context.Background()
	verifier, err := auth.NewVerifier(ctx, cfg.OIDCIssuerURL, cfg.OIDCAudience)
	if err != nil {
		log.Fatalf("initializing token verifier: %v", err)
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("loading AWS config: %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.DynamoDBEndpointURL != "" {
			o.BaseEndpoint = &cfg.DynamoDBEndpointURL
		}
	})
	switchRepo := dynamo.NewSwitchRepository(dynamoClient, cfg.SwitchTableName)
	ecPartRepo := dynamo.NewECPartRepository(dynamoClient, cfg.ECPartTableName)
	keyboardRepo := dynamo.NewKeyboardRepository(dynamoClient, cfg.KeyboardTableName)
	keycapSetRepo := dynamo.NewKeycapSetRepository(dynamoClient, cfg.KeycapSetTableName)
	buildRepo := dynamo.NewBuildRepository(dynamoClient, cfg.BuildTableName)
	profileRepo := dynamo.NewProfileRepository(dynamoClient, cfg.ProfileTableName, cfg.ProfileUsernameTableName)
	uploads := uploadlimit.New(dynamo.NewUploadCounter(dynamoClient, cfg.UploadCounterTableName), cfg.DailyUploadLimit)

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.S3EndpointURL != "" {
			o.BaseEndpoint = &cfg.S3EndpointURL
			// A local emulator needs path-style addressing
			// (bucket.s3.amazonaws.com virtual-hosted-style doesn't resolve
			// against a non-AWS endpoint); real S3 doesn't.
			o.UsePathStyle = true
		}
	})
	// All entities' images currently live in the same bucket.
	presignClient := s3.NewPresignClient(s3Client)

	keyID, key, err := loadImageSigningKey(ctx, ssm.NewFromConfig(awsCfg), cfg.ImagesSigningKeyParam, cfg.ImagesSigningKeyIDParam)
	if err != nil {
		log.Fatalf("loading image signing key: %v", err)
	}
	getSigner := cloudfront.NewGetSigner(cfg.ImagesBaseURL, keyID, key)
	cdn := cloudfront.NewInvalidator(awscloudfront.NewFromConfig(awsCfg), cfg.ImagesDistributionID)

	keycapKitImageStore := imagestore.NewKeycapKitImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	buildImageStore := imagestore.NewBuildImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	keyboardImageStore := imagestore.NewKeyboardImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	switchImageStore := imagestore.NewSwitchImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	ecPartImageStore := imagestore.NewECPartImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	profileImageStore := imagestore.NewProfileImageStore(s3Client, presignClient, cfg.ImagesBucketName, getSigner, cdn)
	ownerImageStore := imagestore.NewOwnerImageStore(s3Client, cfg.ImagesBucketName, cdn)

	var deleteNotices accountdelete.Notifier
	if cfg.AccountDeleteTopicARN != "" {
		deleteNotices = deletenotice.NewSNS(sns.NewFromConfig(awsCfg), cfg.AccountDeleteTopicARN)
	}

	handler := router.New(verifier, switchRepo, switchImageStore, ecPartRepo, ecPartImageStore, keyboardRepo, keyboardImageStore, keycapSetRepo, keycapKitImageStore, buildRepo, buildImageStore, profileRepo, profileImageStore, ownerImageStore, deleteNotices, uploads, cfg.OIDCIssuerURL, cfg.IDPConsentPublicToken, Version)

	// ReadHeaderTimeout bounds a slow/malicious client independently of
	// Lambda's own per-invocation timeout.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting server", "port", port, "version", Version)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// loadImageSigningKey reads the image signing key and its CloudFront public
// key ID in one call, so a key rotation can't pair a new key with an old ID.
func loadImageSigningKey(ctx context.Context, client *ssm.Client, keyParam, keyIDParam string) (string, crypto.Signer, error) {
	out, err := client.GetParameters(ctx, &ssm.GetParametersInput{
		Names:          []string{keyParam, keyIDParam},
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", nil, fmt.Errorf("reading SSM parameters %s and %s: %w", keyParam, keyIDParam, err)
	}
	if len(out.InvalidParameters) > 0 {
		return "", nil, fmt.Errorf("SSM parameters not found: %v", out.InvalidParameters)
	}

	values := make(map[string]string, len(out.Parameters))
	for _, p := range out.Parameters {
		values[aws.ToString(p.Name)] = aws.ToString(p.Value)
	}

	key, err := sign.LoadPEMPrivKeyPKCS8AsSigner(strings.NewReader(values[keyParam]))
	if err != nil {
		return "", nil, fmt.Errorf("parsing %s: %w", keyParam, err)
	}

	return values[keyIDParam], key, nil
}
