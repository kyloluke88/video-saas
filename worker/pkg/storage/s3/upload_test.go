package s3

import (
	"net/http"
	"testing"
	"time"
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
