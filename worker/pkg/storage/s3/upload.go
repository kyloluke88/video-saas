package s3

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	PublicURL string
}

type Client struct {
	cfg Config
	api *awss3.Client
}

type UploadInput struct {
	LocalPath    string
	ObjectKey    string
	ContentType  string
	CacheControl string
}

type UploadResult struct {
	ObjectKey   string
	URL         string
	ContentType string
}

func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.Endpoint == "" {
			return aws.Endpoint{}, &aws.EndpointNotFoundError{}
		}
		return aws.Endpoint{URL: cfg.Endpoint, HostnameImmutable: true}, nil
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, err
	}

	api := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.Endpoint != "" {
			o.UsePathStyle = true
		}
	})
	return &Client{cfg: cfg, api: api}, nil
}

func UploadFile(ctx context.Context, cfg Config, filePath, objectKey string) (string, error) {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return "", err
	}
	result, err := client.UploadFile(ctx, UploadInput{
		LocalPath: filePath,
		ObjectKey: objectKey,
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func DownloadFile(ctx context.Context, cfg Config, objectKey, targetPath string) error {
	client, err := NewClient(ctx, cfg)
	if err != nil {
		return err
	}
	return client.DownloadFile(ctx, objectKey, targetPath)
}

func (c *Client) UploadFile(ctx context.Context, input UploadInput) (UploadResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	localPath := strings.TrimSpace(input.LocalPath)
	objectKey := normalizeObjectKey(input.ObjectKey)
	if localPath == "" {
		return UploadResult{}, fmt.Errorf("local path is required")
	}
	if objectKey == "" {
		return UploadResult{}, fmt.Errorf("object key is required")
	}

	f, err := os.Open(localPath)
	if err != nil {
		return UploadResult{}, err
	}
	defer f.Close()

	contentType := firstNonEmpty(strings.TrimSpace(input.ContentType), contentTypeForPath(localPath), "application/octet-stream")
	req := &awss3.PutObjectInput{
		Bucket:      aws.String(c.cfg.Bucket),
		Key:         aws.String(objectKey),
		Body:        f,
		ContentType: aws.String(contentType),
	}
	if cacheControl := strings.TrimSpace(input.CacheControl); cacheControl != "" {
		req.CacheControl = aws.String(cacheControl)
	}
	if _, err := c.api.PutObject(ctx, req); err != nil {
		return UploadResult{}, err
	}

	return UploadResult{
		ObjectKey:   objectKey,
		URL:         c.ObjectURL(objectKey),
		ContentType: contentType,
	}, nil
}

func (c *Client) DownloadFile(ctx context.Context, objectKey, targetPath string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	objectKey = normalizeObjectKey(objectKey)
	targetPath = strings.TrimSpace(targetPath)
	if objectKey == "" {
		return fmt.Errorf("object key is required")
	}
	if targetPath == "" {
		return fmt.Errorf("target path is required")
	}

	resp, err := c.api.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(c.cfg.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return out.Close()
}

func (c *Client) ObjectURL(objectKey string) string {
	objectKey = normalizeObjectKey(objectKey)
	if objectKey == "" {
		return ""
	}
	if c.cfg.PublicURL != "" {
		return strings.TrimRight(c.cfg.PublicURL, "/") + "/" + objectKey
	}
	if c.cfg.Endpoint == "" && c.cfg.Region != "" {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", c.cfg.Bucket, c.cfg.Region, objectKey)
	}
	return fmt.Sprintf("s3://%s/%s", c.cfg.Bucket, objectKey)
}

func normalizeConfig(cfg Config) Config {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Region = firstNonEmpty(strings.TrimSpace(cfg.Region), "us-east-1")
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.AccessKey = strings.TrimSpace(cfg.AccessKey)
	cfg.SecretKey = strings.TrimSpace(cfg.SecretKey)
	cfg.PublicURL = strings.TrimSpace(cfg.PublicURL)
	return cfg
}

func normalizeObjectKey(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimLeft(value, "/")
	value = strings.ReplaceAll(value, "\\", "/")
	return value
}

func contentTypeForPath(path string) string {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(strings.TrimSpace(path))))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
