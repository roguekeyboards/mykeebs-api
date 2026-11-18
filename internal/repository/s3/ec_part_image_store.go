package s3

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/roguekeyboards/mykeebs-api/internal/log"
	"github.com/roguekeyboards/mykeebs-api/internal/repository"
	"github.com/roguekeyboards/mykeebs-api/internal/repository/cloudfront"
)

// ECPartImageStore is the S3-backed repository.ECPartImageStore.
type ECPartImageStore struct {
	client  s3API
	presign s3PresignAPI
	bucket  string
	get     getSigner
	cdn     invalidator
}

var _ repository.ECPartImageStore = (*ECPartImageStore)(nil)

// NewECPartImageStore returns a ECPartImageStore backed by client.
func NewECPartImageStore(client *s3.Client, presign *s3.PresignClient, bucket string, get *cloudfront.GetSigner, cdn *cloudfront.Invalidator) *ECPartImageStore {
	return &ECPartImageStore{
		client:  client,
		presign: presign,
		bucket:  bucket,
		get:     get,
		cdn:     cdn,
	}
}

// SignGet implements repository.ECPartImageStore.
func (s *ECPartImageStore) SignGet(_ context.Context, key repository.ECPartImageKey) (url string, err error) {
	return s.get.Sign(string(key))
}

// PresignPut implements repository.ECPartImageStore.
func (s *ECPartImageStore) PresignPut(ctx context.Context, key repository.ECPartImageKey, contentType string, size int64) (string, error) {
	req, err := s.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(string(key)),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
		IfNoneMatch:   aws.String("*"),
	})
	if err != nil {
		return "", fmt.Errorf("presigning PUT s3://%s/%s (content-type %q, %d bytes): %w", s.bucket, key, contentType, size, err)
	}

	return req.URL, nil
}

// Delete implements repository.ECPartImageStore.
func (s *ECPartImageStore) Delete(ctx context.Context, key repository.ECPartImageKey) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(string(key)),
	})
	if err != nil {
		return fmt.Errorf("deleting s3://%s/%s: %w", s.bucket, key, err)
	}

	if err := s.cdn.Invalidate(ctx, "/"+string(key)); err != nil {
		log.FromContext(ctx).Error("invalidating a deleted image", log.Error, err)
	}

	return nil
}
