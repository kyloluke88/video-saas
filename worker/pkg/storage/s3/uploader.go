package s3

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PublicURL string
}

func UploadFile(cfg Config, filePath, objectKey string) (string, error) {
	ctx := context.Background()
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.Endpoint == "" {
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		}
		return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
	})

	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		config.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.UsePathStyle = true
		}
	})

	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(objectKey),
		Body:        f,
		ContentType: aws.String("video/mp4"),
	})
	if err != nil {
		return "", err
	}

	if cfg.PublicURL != "" {
		return strings.TrimRight(cfg.PublicURL, "/") + "/" + objectKey, nil
	}
	return fmt.Sprintf("s3://%s/%s", cfg.Bucket, objectKey), nil
}
