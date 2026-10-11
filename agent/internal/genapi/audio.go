package genapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// sniffAudioFormat identifies the audio container of a raw payload by magic
// bytes. Generation artifacts that are not recognizable audio fail closed
// (ClassUnsupportedFormat) so garbage never reaches the ingest path.
func sniffAudioFormat(raw []byte) (string, error) {
	if len(raw) < 12 {
		return "", errors.New("artifact too small to be audio")
	}
	switch {
	case string(raw[0:4]) == "RIFF" && string(raw[8:12]) == "WAVE":
		return "wav", nil
	case string(raw[0:3]) == "ID3":
		return "mp3", nil
	case len(raw) > 2 && raw[0] == 0xFF && (raw[1]&0xE0) == 0xE0:
		return "mp3", nil
	case string(raw[0:4]) == "OggS":
		return "ogg", nil
	case string(raw[0:4]) == "fLaC":
		return "flac", nil
	case string(raw[0:4]) == "MThd":
		return "mid", nil
	}
	return "", fmt.Errorf("unrecognized audio format (magic % x)", raw[0:6])
}

// decodeBase64Flexible accepts padded, unpadded and data-URI-wrapped base64.
func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "base64,"); idx >= 0 && strings.HasPrefix(s, "data:") {
		s = s[idx+len("base64,"):]
	}
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', ' ', '\t':
			return -1
		}
		return r
	}, s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

// sanitizeFileStem keeps artifact filenames filesystem-safe.
func sanitizeFileStem(stem string) string {
	stem = unsafeFileChars.ReplaceAllString(strings.TrimSpace(stem), "_")
	stem = strings.Trim(stem, "._-")
	if stem == "" {
		stem = "artifact"
	}
	if len(stem) > 96 {
		stem = stem[:96]
	}
	return stem
}

// resolvedDownloadDir anchors the cache under ~/.vit unless configured
// otherwise; runtime state never lands in the source tree.
func resolvedDownloadDir(providerID, configured string) string {
	if strings.TrimSpace(configured) != "" {
		return strings.TrimSpace(configured)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(os.TempDir(), "vit-genapi")
	}
	return filepath.Join(home, ".vit", "genapi-cache", sanitizeFileStem(providerID))
}

func joinDownload(dir, name string) string {
	return filepath.Join(dir, name)
}

// writeExclusively creates the download dir and writes without clobbering
// an existing different file (same-name retries stay idempotent by size).
func writeExclusively(name string, raw []byte) error {
	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create download dir %s: %w", dir, err)
	}
	if info, err := os.Stat(name); err == nil {
		if info.Size() == int64(len(raw)) {
			return nil
		}
		return fmt.Errorf("artifact path already exists with different size: %s", name)
	}
	if err := os.WriteFile(name, raw, 0o644); err != nil {
		return fmt.Errorf("write artifact %s: %w", name, err)
	}
	return nil
}
