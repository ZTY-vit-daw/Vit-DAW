package vps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BuildInstallationFingerprint returns a content-addressed fingerprint for a
// plug-in package on the local machine. A credential is intentionally not
// issued when the package cannot be read: a name, version string or path by
// itself is not strong enough to detect an in-place plug-in replacement.
//
// Both single-file plug-ins and directory packages (for example a VST3
// bundle) are supported. Symbolic links are rejected so the fingerprint is
// always tied to the package observed at the supplied path.
func BuildInstallationFingerprint(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("plugin installation path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve plugin installation path: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat plugin installation: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("plugin installation path must not be a symbolic link")
	}

	hash := sha256.New()
	_, _ = io.WriteString(hash, "vit.vps.installation.v1\n")
	if info.IsDir() {
		files, err := installationFiles(absolute)
		if err != nil {
			return "", err
		}
		if len(files) == 0 {
			return "", fmt.Errorf("plugin installation directory contains no regular files")
		}
		for _, file := range files {
			if err := appendInstallationFileHash(hash, absolute, file); err != nil {
				return "", err
			}
		}
	} else if info.Mode().IsRegular() {
		if err := appendInstallationFileHash(hash, filepath.Dir(absolute), absolute); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("plugin installation must be a regular file or directory")
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// BuildFileFingerprint returns the raw content SHA-256 for one regular
// plug-in file.  It intentionally has no package header: this is the exact
// file-fingerprint grammar emitted by the independent Windows VST3 worker for
// single-file .vst3 installations.  Directory bundles must use
// BuildInstallationFingerprint instead.
func BuildFileFingerprint(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("plugin file path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve plugin file path: %w", err)
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", fmt.Errorf("stat plugin file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("plugin file path must not be a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("plugin file must be a regular file")
	}
	file, err := os.Open(absolute)
	if err != nil {
		return "", fmt.Errorf("open plugin file: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash plugin file: %w", err)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func installationFiles(root string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin installation contains symbolic link %s", path)
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk plugin installation: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func appendInstallationFileHash(destination io.Writer, root, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat plug-in package file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("plug-in package member is not a regular file: %s", path)
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve plugin package member: %w", err)
	}
	relative = filepath.ToSlash(relative)
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open plug-in package file: %w", err)
	}
	defer file.Close()
	fileHash := sha256.New()
	if _, err := io.Copy(fileHash, file); err != nil {
		return fmt.Errorf("hash plug-in package file: %w", err)
	}
	_, _ = fmt.Fprintf(destination, "%s\x00%d\x00%s\n", relative, info.Size(), hex.EncodeToString(fileHash.Sum(nil)))
	return nil
}
