package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const serverStorageManifestVersion = 1

type serverStorageManifest struct {
	Version  int    `json:"version"`
	ServerID string `json:"serverId"`
	DiskMB   int64  `json:"diskMb"`
}

// serverStorageLayout establishes a durable, identity-bound directory for a
// server. It deliberately does not mount the directory into Docker: quota
// enforcement and mount verification must be implemented before provisioning
// can safely depend on this layout.
type serverStorageLayout struct {
	root string
}

func newServerStorageLayout(root string) (*serverStorageLayout, error) {
	root = strings.TrimSpace(root)
	if !filepath.IsAbs(root) || filepath.Clean(root) == string(filepath.Separator) {
		return nil, errors.New("server storage root must be an absolute non-root path")
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect server storage root: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("server storage root must be a real directory, not a symlink")
	}
	if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("server storage root must not be accessible by group or other users")
	}
	return &serverStorageLayout{root: filepath.Clean(root)}, nil
}

func (s *serverStorageLayout) ensure(serverID string, diskMB int64) (string, error) {
	if s == nil || s.root == "" {
		return "", errors.New("server storage layout is not configured")
	}
	if !serverIDPattern.MatchString(serverID) {
		return "", errors.New("server identity is invalid")
	}
	if diskMB < 512 || diskMB > 16_777_216 {
		return "", errors.New("server disk allocation is outside supported bounds")
	}

	dir := filepath.Join(s.root, serverID)
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return "", fmt.Errorf("create server storage directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", fmt.Errorf("inspect server storage directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("server storage path must be a real directory, not a symlink")
	}
	if info.Mode().Perm()&0077 != 0 {
		return "", errors.New("server storage directory permissions are too broad")
	}

	manifestPath := filepath.Join(dir, "vampire-storage.json")
	manifestInfo, statErr := os.Lstat(manifestPath)
	if statErr == nil {
		if !manifestInfo.Mode().IsRegular() || manifestInfo.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("server storage manifest must be a regular file")
		}
		data, readErr := os.ReadFile(manifestPath)
		if readErr != nil {
			return "", fmt.Errorf("read server storage manifest: %w", readErr)
		}
		var existing serverStorageManifest
		if err := json.Unmarshal(data, &existing); err != nil {
			return "", errors.New("server storage manifest is invalid")
		}
		if existing.Version != serverStorageManifestVersion || existing.ServerID != serverID || existing.DiskMB != diskMB {
			return "", errors.New("server storage manifest does not match requested identity and allocation")
		}
		return dir, nil
	}
	if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("inspect server storage manifest: %w", statErr)
	}

	manifest := serverStorageManifest{Version: serverStorageManifestVersion, ServerID: serverID, DiskMB: diskMB}
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", errors.New("encode server storage manifest")
	}
	tmp, err := os.CreateTemp(dir, ".vampire-storage-*")
	if err != nil {
		return "", fmt.Errorf("create storage manifest temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("secure storage manifest: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return "", fmt.Errorf("write storage manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync storage manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close storage manifest: %w", err)
	}
	if err := os.Rename(tmpName, manifestPath); err != nil {
		return "", fmt.Errorf("publish storage manifest: %w", err)
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return "", fmt.Errorf("open server storage directory: %w", err)
	}
	defer dirFile.Close()
	if err := dirFile.Sync(); err != nil {
		return "", fmt.Errorf("sync server storage directory: %w", err)
	}
	return dir, nil
}
