package s3

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func TestNewHTTPClientTransportTimeouts(t *testing.T) {
	client := newHTTPClient()
	if client == nil {
		t.Fatal("expected http client")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type %T", client.Transport)
	}

	if transport.TLSHandshakeTimeout < 45*time.Second {
		t.Fatalf("expected TLS handshake timeout >= 45s, got %s", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout < 60*time.Second {
		t.Fatalf("expected response header timeout >= 60s, got %s", transport.ResponseHeaderTimeout)
	}
	if transport.IdleConnTimeout < 90*time.Second {
		t.Fatalf("expected idle conn timeout >= 90s, got %s", transport.IdleConnTimeout)
	}
	if client.Timeout != 0 {
		t.Fatalf("expected no global client timeout, got %s", client.Timeout)
	}
}

type fakeS3API struct {
	headErr     error
	putErrs     []error
	putCalls    int
	deleteErr   error
	deleteCalls int
}

func (f *fakeS3API) PutObject(_ context.Context, _ *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	f.putCalls++
	if len(f.putErrs) == 0 {
		return &awss3.PutObjectOutput{}, nil
	}
	err := f.putErrs[0]
	f.putErrs = f.putErrs[1:]
	if err != nil {
		return nil, err
	}
	return &awss3.PutObjectOutput{}, nil
}

func (f *fakeS3API) GetObject(_ context.Context, _ *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	return &awss3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(""))}, nil
}

func (f *fakeS3API) HeadObject(_ context.Context, _ *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	if f.headErr != nil {
		return nil, f.headErr
	}
	return &awss3.HeadObjectOutput{}, nil
}

func (f *fakeS3API) DeleteObject(_ context.Context, _ *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	f.deleteCalls++
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return &awss3.DeleteObjectOutput{}, nil
}

func TestClientUploadFileDeletesAndRetriesWhenOverwriteFails(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "demo.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write local file failed: %v", err)
	}

	api := &fakeS3API{
		putErrs: []error{&smithy.GenericAPIError{Code: "Conflict", Message: "already exists"}, nil},
	}
	client := &Client{
		cfg: Config{Bucket: "demo-bucket"},
		api: api,
	}

	result, err := client.UploadFile(context.Background(), UploadInput{
		LocalPath: localPath,
		ObjectKey: "podcast/demo.txt",
	})
	if err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if api.putCalls != 2 {
		t.Fatalf("expected 2 put calls, got %d", api.putCalls)
	}
	if api.deleteCalls != 1 {
		t.Fatalf("expected 1 delete call, got %d", api.deleteCalls)
	}
	if result.URL != "s3://demo-bucket/podcast/demo.txt" {
		t.Fatalf("unexpected url: %s", result.URL)
	}
}

func TestClientUploadFileSkipsDeleteWhenObjectDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	localPath := filepath.Join(dir, "demo.txt")
	if err := os.WriteFile(localPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write local file failed: %v", err)
	}

	api := &fakeS3API{
		headErr: &smithy.GenericAPIError{Code: "NotFound", Message: "missing"},
		putErrs: []error{nil},
	}
	client := &Client{
		cfg: Config{Bucket: "demo-bucket"},
		api: api,
	}

	if _, err := client.UploadFile(context.Background(), UploadInput{
		LocalPath: localPath,
		ObjectKey: "podcast/demo.txt",
	}); err != nil {
		t.Fatalf("UploadFile failed: %v", err)
	}
	if api.putCalls != 1 {
		t.Fatalf("expected 1 put call, got %d", api.putCalls)
	}
	if api.deleteCalls != 0 {
		t.Fatalf("expected 0 delete calls, got %d", api.deleteCalls)
	}
}
