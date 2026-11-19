// Package account seeds one of each item a profile delete must remove,
// plus image objects no record points to, for the REST and MCP account
// delete specs.
package account

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"

	"github.com/roguekeyboards/mykeebs-api/test/functional/support"
	"github.com/roguekeyboards/mykeebs-api/test/functional/support/db"
)

// Items is what Seed created.
type Items struct {
	KeyboardID, SwitchID, ECPartID, KeycapSetID, BuildID string

	// StrayImageKeys are objects under the owner's prefixes that no record
	// points to, so the moderation function skips them.
	StrayImageKeys []string
}

// Seed creates a keyboard and a switch with images, an EC part, a keycap
// set with a kit, a build referencing the keyboard, and three stray image
// objects.
func Seed(ctx context.Context, ownerID string) (Items, error) {
	items := Items{
		KeyboardID:  uuid.NewString(),
		SwitchID:    uuid.NewString(),
		ECPartID:    uuid.NewString(),
		KeycapSetID: uuid.NewString(),
		BuildID:     uuid.NewString(),
	}
	items.StrayImageKeys = []string{
		"keyboards/" + ownerID + "/" + items.KeyboardID + "/images/" + uuid.NewString(),
		"profiles/" + ownerID + "/avatar/" + uuid.NewString(),
		"ec-parts/" + ownerID + "/" + items.ECPartID + "/image/" + uuid.NewString(),
	}

	err := errors.Join(
		db.SeedKeyboardWithImage(ctx, ownerID, items.KeyboardID, uuid.NewString(), "private"),
		db.SeedSwitchWithImage(ctx, ownerID, items.SwitchID, "public"),
		db.SeedECPart(ctx, ownerID, items.ECPartID, "public"),
		db.SeedKeycapSetWithKit(ctx, ownerID, items.KeycapSetID, uuid.NewString(), "private"),
		db.SeedBuild(ctx, ownerID, items.BuildID, items.KeyboardID, "public"),
	)
	if err != nil {
		return items, err
	}

	client, err := s3Client(ctx)
	if err != nil {
		return items, err
	}
	for _, key := range items.StrayImageKeys {
		if _, err := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(support.ImagesBucketName()),
			Key:    aws.String(key),
			Body:   strings.NewReader("stray"),
		}); err != nil {
			return items, err
		}
	}
	return items, nil
}

// CleanUp removes what Seed created, best-effort, for a spec that failed
// before the delete under test landed.
func (i Items) CleanUp(ctx context.Context, ownerID string) {
	_ = db.DeleteBuild(ctx, ownerID, i.BuildID, i.KeyboardID)
	_ = db.DeleteKeyboard(ctx, ownerID, i.KeyboardID)
	_ = db.DeleteSwitch(ctx, ownerID, i.SwitchID)
	_ = db.DeleteECPart(ctx, ownerID, i.ECPartID)
	_ = db.DeleteKeycapSet(ctx, ownerID, i.KeycapSetID)
	if client, err := s3Client(ctx); err == nil {
		for _, key := range i.StrayImageKeys {
			_, _ = client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(support.ImagesBucketName()),
				Key:    aws.String(key),
			})
		}
	}
}

// RemainingStrayImages returns the stray image keys still in the bucket.
func (i Items) RemainingStrayImages(ctx context.Context) ([]string, error) {
	client, err := s3Client(ctx)
	if err != nil {
		return nil, err
	}
	var remaining []string
	for _, key := range i.StrayImageKeys {
		_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(support.ImagesBucketName()),
			Key:    aws.String(key),
		})
		if _, ok := errors.AsType[*s3types.NotFound](err); ok {
			continue
		}
		if err != nil {
			return nil, err
		}
		remaining = append(remaining, key)
	}
	return remaining, nil
}

func s3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := support.S3EndpointURL()
	if endpoint != "" {
		cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
	}
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	}), nil
}
