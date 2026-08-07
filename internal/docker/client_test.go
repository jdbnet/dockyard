package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestPublicPortStrings_dedupesIPv4IPv6(t *testing.T) {
	ports := publicPortStrings([]container.Port{
		{IP: "0.0.0.0", PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
		{IP: "::", PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
	})
	if len(ports) != 1 {
		t.Fatalf("expected 1 port, got %v", ports)
	}
	if ports[0] != "8080:80/tcp" {
		t.Fatalf("unexpected port %q", ports[0])
	}
}

func TestPublicPortStrings_keepsDistinctBindings(t *testing.T) {
	ports := publicPortStrings([]container.Port{
		{IP: "0.0.0.0", PublicPort: 8080, PrivatePort: 80, Type: "tcp"},
		{IP: "0.0.0.0", PublicPort: 8443, PrivatePort: 443, Type: "tcp"},
		{IP: "127.0.0.1", PublicPort: 3000, PrivatePort: 3000, Type: "tcp"},
	})
	if len(ports) != 3 {
		t.Fatalf("expected 3 ports, got %v", ports)
	}
}

func TestPublicPortStrings_skipsUnpublished(t *testing.T) {
	ports := publicPortStrings([]container.Port{
		{PrivatePort: 5432, Type: "tcp"},
	})
	if len(ports) != 0 {
		t.Fatalf("expected no ports, got %v", ports)
	}
}
