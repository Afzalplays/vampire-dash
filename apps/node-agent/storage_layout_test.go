package main

import (
	"os"
	"path/filepath"
	"testing"
)

func storageTestRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "servers")
	if err := os.Mkdir(root, 0700); err != nil { t.Fatal(err) }
	return root
}

func TestServerStorageLayoutCreatesDurableIdentityManifest(t *testing.T) {
	root := storageTestRoot(t)
	layout, err := newServerStorageLayout(root)
	if err != nil { t.Fatal(err) }
	got, err := layout.ensure("server_123", 2048)
	if err != nil { t.Fatal(err) }
	if got != filepath.Join(root, "server_123") { t.Fatalf("unexpected path %q", got) }
	info, err := os.Stat(filepath.Join(got, "vampire-storage.json"))
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0600 { t.Fatalf("manifest mode = %o, want 600", info.Mode().Perm()) }
	if _, err := layout.ensure("server_123", 2048); err != nil { t.Fatalf("idempotent ensure: %v", err) }
}

func TestServerStorageLayoutRejectsIdentityOrQuotaDrift(t *testing.T) {
	layout, err := newServerStorageLayout(storageTestRoot(t))
	if err != nil { t.Fatal(err) }
	if _, err := layout.ensure("server_123", 2048); err != nil { t.Fatal(err) }
	if _, err := layout.ensure("server_123", 4096); err == nil { t.Fatal("expected changed allocation to be rejected") }
}

func TestServerStorageLayoutRejectsUnsafePathsAndIDs(t *testing.T) {
	root := storageTestRoot(t)
	if _, err := newServerStorageLayout("/"); err == nil { t.Fatal("expected root path rejection") }
	if _, err := newServerStorageLayout(filepath.Join(root, "missing")); err == nil { t.Fatal("expected missing root rejection") }
	layout, err := newServerStorageLayout(root)
	if err != nil { t.Fatal(err) }
	for _, id := range []string{"../escape", ".", "", "a/b"} {
		if _, err := layout.ensure(id, 2048); err == nil { t.Errorf("expected invalid ID %q to be rejected", id) }
	}
}

func TestServerStorageLayoutRejectsSymlinkedServerDirectory(t *testing.T) {
	root := storageTestRoot(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "server_link")); err != nil { t.Fatal(err) }
	layout, err := newServerStorageLayout(root)
	if err != nil { t.Fatal(err) }
	if _, err := layout.ensure("server_link", 2048); err == nil { t.Fatal("expected symlink rejection") }
}
