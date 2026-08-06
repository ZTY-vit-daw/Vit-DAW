package processorattestation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FingerprintPath hashes a plug-in file by content. VST3 bundles are hashed as
// a sorted, framed sequence of relative file paths and file bytes.
func FingerprintPath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return "", fmt.Errorf("processor attestation: plug-in path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("processor attestation: inspect plug-in path: %w", err)
	}
	h := sha256.New()
	if info.Mode().IsRegular() {
		if err := hashFileBytes(h, path); err != nil {
			return "", err
		}
		return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
	}
	if !info.IsDir() {
		return "", fmt.Errorf("processor attestation: plug-in path must be a regular file or directory")
	}
	files := []string{}
	err = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("processor attestation: bundle contains symlink %q", current)
		}
		if entry.Type().IsRegular() {
			files = append(files, current)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("processor attestation: walk plug-in bundle: %w", err)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("processor attestation: plug-in bundle contains no regular files")
	}
	sort.Strings(files)
	_, _ = h.Write([]byte("vit-pca-bundle-v1\x00"))
	for _, file := range files {
		relative, err := filepath.Rel(path, file)
		if err != nil {
			return "", fmt.Errorf("processor attestation: relative bundle path: %w", err)
		}
		if err := hashFrame(h, []byte(filepath.ToSlash(relative))); err != nil {
			return "", err
		}
		if err := hashFileFrame(h, file); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

func hashFileBytes(destination hash.Hash, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("processor attestation: open plug-in binary: %w", err)
	}
	defer file.Close()
	if _, err := io.Copy(destination, file); err != nil {
		return fmt.Errorf("processor attestation: hash plug-in binary: %w", err)
	}
	return nil
}

func hashFileFrame(destination hash.Hash, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("processor attestation: stat bundle file: %w", err)
	}
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(info.Size()))
	if _, err := destination.Write(size[:]); err != nil {
		return err
	}
	return hashFileBytes(destination, path)
}

func hashFrame(destination hash.Hash, value []byte) error {
	var size [8]byte
	binary.BigEndian.PutUint64(size[:], uint64(len(value)))
	if _, err := destination.Write(size[:]); err != nil {
		return err
	}
	_, err := destination.Write(value)
	return err
}
