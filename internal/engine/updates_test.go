package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCheckableImageRef(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"nginx:1.27", true},
		{"ghcr.io/jdbnet/dockyard:latest", true},
		{"nginx", true},
		{"registry.example.com:5000/app:1", true},
		{"nginx@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"nginx:1.27@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		{"<none>:<none>", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := checkableImageRef(tc.ref); got != tc.want {
			t.Fatalf("checkableImageRef(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

func TestUpdateAvailable(t *testing.T) {
	local := []string{"nginx@sha256:aaa"}
	if updateAvailable("sha256:aaa", local) {
		t.Fatal("matching digest should not be an update")
	}
	if !updateAvailable("sha256:bbb", local) {
		t.Fatal("different digest should be an update")
	}
	if updateAvailable("", local) {
		t.Fatal("empty remote digest is not an update")
	}
	if !updateAvailable("sha256:bbb", nil) {
		t.Fatal("missing local digests should be an update")
	}
}

func TestLocalDigestForRef(t *testing.T) {
	digests := []string{
		"redis@sha256:111",
		"docker.io/library/nginx@sha256:222",
	}
	if got := localDigestForRef("nginx:1.27", digests); got != "sha256:222" {
		t.Fatalf("got %q", got)
	}
	if got := localDigestForRef("caddy:alpine", digests); got != "sha256:111" {
		t.Fatalf("fallback got %q", got)
	}
}

func TestCloneUpdatesKeepsEmptySlices(t *testing.T) {
	got := cloneUpdates(UpdatesReport{
		Pending:  []ImageUpdate{},
		Failures: []ImageUpdate{},
	})
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "null") {
		t.Fatalf("empty slices encoded as null: %s", raw)
	}
}
