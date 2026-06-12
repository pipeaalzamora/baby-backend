// Package storage contains object storage integrations.
package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Service wraps the S3 client and presigner used by media handlers.
type S3Service struct {
	bucket    string
	client    *s3.Client
	presigner *s3.PresignClient
	ttl       time.Duration
}

// NewS3Service creates an S3 service. If bucket is empty, storage is disabled.
func NewS3Service(ctx context.Context, region, bucket string, ttl time.Duration) (*S3Service, error) {
	if bucket == "" {
		return nil, nil
	}
	if region == "" {
		return nil, errors.New("AWS_REGION requerido para S3")
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg)
	return &S3Service{
		bucket:    bucket,
		client:    client,
		presigner: s3.NewPresignClient(client),
		ttl:       ttl,
	}, nil
}

// Enabled reports whether S3 is configured.
func (s *S3Service) Enabled() bool {
	return s != nil && s.bucket != "" && s.presigner != nil
}

// Bucket returns the configured media bucket.
func (s *S3Service) Bucket() string {
	if s == nil {
		return ""
	}
	return s.bucket
}

// PresignPut creates a short-lived URL for direct browser uploads.
func (s *S3Service) PresignPut(ctx context.Context, key, contentType string) (string, time.Time, error) {
	if !s.Enabled() {
		return "", time.Time{}, errors.New("S3 no configurado")
	}

	expiresAt := time.Now().Add(s.ttl)
	req, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return "", time.Time{}, err
	}
	return req.URL, expiresAt, nil
}

// PresignGet creates a short-lived URL for reading a private object.
func (s *S3Service) PresignGet(ctx context.Context, bucket, key string) (string, error) {
	if !s.Enabled() {
		return "", errors.New("S3 no configurado")
	}
	if bucket == "" {
		bucket = s.bucket
	}
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(s.ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Delete removes an object from S3.
func (s *S3Service) Delete(ctx context.Context, bucket, key string) error {
	if !s.Enabled() || key == "" {
		return nil
	}
	if bucket == "" {
		bucket = s.bucket
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeletePrefix removes every object below a private account prefix.
func (s *S3Service) DeletePrefix(ctx context.Context, prefix string) error {
	if !s.Enabled() {
		return nil
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}

		objects := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, object := range page.Contents {
			if object.Key == nil || *object.Key == "" {
				continue
			}
			objects = append(objects, types.ObjectIdentifier{Key: object.Key})
		}
		if len(objects) == 0 {
			continue
		}

		_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
