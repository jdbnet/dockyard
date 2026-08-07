package updater

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinaryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		arch    string
		want    string
		wantErr bool
	}{
		{"amd64", "dockyard-amd64", false},
		{"arm64", "dockyard-arm64", false},
		{"386", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.arch, func(t *testing.T) {
			got, err := binaryName(tt.arch)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("binaryName() error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("binaryName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{" 2.0.0 ", "v2.0.0"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := normalizeVersion(tt.in); got != tt.want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestShouldApplyUpdate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		local  string
		remote string
		want   bool
	}{
		{"1.0.0", "1.1.0", true},
		{"1.1.0", "1.0.0", false},
		{"1.2.3", "1.2.3", true},
		{"dev", "1.0.0", false},
		{"1.0.0", "dev", false},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s_to_%s", tt.local, tt.remote)
		t.Run(name, func(t *testing.T) {
			if got := shouldApplyUpdate(tt.local, tt.remote); got != tt.want {
				t.Fatalf("shouldApplyUpdate(%q, %q) = %v, want %v", tt.local, tt.remote, got, tt.want)
			}
		})
	}
}

func TestMaybeUpdateWithMockServer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("self-update tests require linux")
	}

	currentBinary := mustBuildTestBinary(t, "1.0.0")

	dir := t.TempDir()
	exePath := filepath.Join(dir, "dockyard-test")
	if err := copyFile(currentBinary, exePath); err != nil {
		t.Fatalf("copy binary: %v", err)
	}
	if err := os.Chmod(exePath, 0o755); err != nil {
		t.Fatalf("chmod binary: %v", err)
	}

	const etagV1 = "abc123"
	const etagV2 = "def456"
	remoteVersion := "99.99.99"
	remotePath := mustBuildTestBinary(t, remoteVersion)
	state := struct {
		etag string
		body []byte
	}{
		etag: etagV1,
		body: mustReadBinary(t, remotePath),
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, state.etag))
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, state.etag))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(state.body)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()

	oldBaseURL := updateBaseURL
	oldClient := httpClient
	oldResolve := resolveExecutablePath
	oldExec := execSelf
	updateBaseURL = srv.URL
	httpClient = srv.Client()
	resolveExecutablePath = func() (string, error) { return exePath, nil }
	execCalled := false
	execSelf = func(path string, args []string, env []string) error {
		execCalled = true
		if path != exePath {
			t.Fatalf("exec path = %q, want %q", path, exePath)
		}
		return nil
	}
	t.Cleanup(func() {
		updateBaseURL = oldBaseURL
		httpClient = oldClient
		resolveExecutablePath = oldResolve
		execSelf = oldExec
	})

	origArgs := os.Args
	os.Args = []string{exePath, "--version"}
	t.Cleanup(func() { os.Args = origArgs })

	if err := maybeUpdate("1.0.0"); err != nil {
		t.Fatalf("maybeUpdate() first run: %v", err)
	}
	if !execCalled {
		t.Fatal("expected exec after first update")
	}
	execCalled = false

	gotVersion, err := binaryVersion(exePath)
	if err != nil {
		t.Fatalf("binaryVersion after update: %v", err)
	}
	if gotVersion != remoteVersion {
		t.Fatalf("updated binary version = %q, want %q", gotVersion, remoteVersion)
	}

	etagBytes, err := os.ReadFile(filepath.Join(dir, etagFileName))
	if err != nil {
		t.Fatalf("read etag file: %v", err)
	}
	if string(etagBytes) != etagV1 {
		t.Fatalf("cached etag = %q, want %q", string(etagBytes), etagV1)
	}

	state.etag = etagV2
	state.body = mustReadBinary(t, mustBuildTestBinary(t, "100.0.0"))
	if err := os.WriteFile(filepath.Join(dir, etagFileName), []byte(etagV1), 0o644); err != nil {
		t.Fatalf("seed etag file: %v", err)
	}

	if err := maybeUpdate(remoteVersion); err != nil {
		t.Fatalf("maybeUpdate() second run: %v", err)
	}
	if !execCalled {
		t.Fatal("expected exec after hotfix update")
	}

	gotVersion, err = binaryVersion(exePath)
	if err != nil {
		t.Fatalf("binaryVersion after hotfix: %v", err)
	}
	if gotVersion != "100.0.0" {
		t.Fatalf("hotfix binary version = %q, want %q", gotVersion, "100.0.0")
	}
}

func TestMaybeUpdateSkipsWhenNotWritable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("self-update tests require linux")
	}
	if os.Geteuid() == 0 {
		t.Skip("requires non-root to test non-writable binary")
	}

	currentBinary, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable(): %v", err)
	}

	dir := t.TempDir()
	exePath := filepath.Join(dir, "dockyard-readonly")
	if err := copyFile(currentBinary, exePath); err != nil {
		t.Fatalf("copy binary: %v", err)
	}
	if err := os.Chmod(exePath, 0o555); err != nil {
		t.Fatalf("chmod binary: %v", err)
	}

	origArgs := os.Args
	os.Args = []string{exePath}
	oldResolve := resolveExecutablePath
	resolveExecutablePath = func() (string, error) { return exePath, nil }
	t.Cleanup(func() {
		os.Args = origArgs
		resolveExecutablePath = oldResolve
	})

	if err := maybeUpdate("1.0.0"); err != nil {
		t.Fatalf("maybeUpdate() on readonly binary: %v", err)
	}
}

func mustBuildTestBinary(t *testing.T, version string) string {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "dockyard-remote")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.Version="+version, "-o", outPath, "./internal/updater/testdata/versionmain")
	cmd.Dir = mustFindModuleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build test binary: %v\n%s", err, out)
	}
	return outPath
}

func mustReadBinary(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	return data
}

func mustFindModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd(): %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o755)
}

func TestFetchETag(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			http.Error(w, "expected HEAD", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("ETag", `"test-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	etag, err := fetchETag(srv.URL)
	if err != nil {
		t.Fatalf("fetchETag() error: %v", err)
	}
	if etag != "test-etag" {
		t.Fatalf("fetchETag() = %q, want %q", etag, "test-etag")
	}
}

func TestBinaryVersion(t *testing.T) {
	path := mustBuildTestBinary(t, "1.2.3")

	version, err := binaryVersion(path)
	if err != nil {
		t.Fatalf("binaryVersion() error: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("binaryVersion() = %q, want %q", version, "1.2.3")
	}
}
