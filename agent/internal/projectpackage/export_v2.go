package projectpackage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/projectstore"
)

const (
	ExportManifestSchemaV2    = "vit_project_package_manifest.v2"
	MediaPolicyReferenceOnly  = "reference_only"
	MediaPolicyCopyReferenced = "copy_referenced_audio"
)

type ExportMediaSource struct {
	SourcePath string
	TargetPath string
	Size       int64
}

type ExportMediaEntry struct {
	Kind       string `json:"kind"`
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path,omitempty"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256,omitempty"`
	Policy     string `json:"policy"`
	Status     string `json:"status"`
}

type ExportManifestV2 struct {
	SchemaVersion              string             `json:"schema_version"`
	ProjectUUID                string             `json:"project_uuid"`
	SourceProjectUUID          string             `json:"source_project_uuid"`
	OriginProjectUUID          string             `json:"origin_project_uuid"`
	ProjectFile                string             `json:"project_file"`
	ExportKind                 string             `json:"export_kind"`
	MediaPolicy                string             `json:"media_policy"`
	AudioSelfContained         bool               `json:"audio_self_contained"`
	UnbundledDependencyClasses []string           `json:"unbundled_dependency_classes"`
	PackageStatus              string             `json:"package_status"`
	CreatedAt                  time.Time          `json:"created_at"`
	Media                      []ExportMediaEntry `json:"media"`
}

type PublishFolderOptions struct {
	SourceProjectPath  string
	SourceProjectUUID  string
	TargetProjectPath  string
	TargetProjectUUID  string
	StagingProjectPath string
	StagingDirectory   string
	TargetDirectory    string
	MediaPolicy        string
	Media              []ExportMediaSource
}

func PublishFolderV2(opts PublishFolderOptions) (ExportManifestV2, error) {
	opts.SourceProjectPath = cleanAbs(opts.SourceProjectPath)
	opts.TargetProjectPath = cleanAbs(opts.TargetProjectPath)
	opts.StagingProjectPath = cleanAbs(opts.StagingProjectPath)
	opts.StagingDirectory = cleanAbs(opts.StagingDirectory)
	opts.TargetDirectory = cleanAbs(opts.TargetDirectory)
	opts.SourceProjectUUID = safeName(opts.SourceProjectUUID)
	opts.TargetProjectUUID = safeName(opts.TargetProjectUUID)
	opts.MediaPolicy = strings.ToLower(strings.TrimSpace(opts.MediaPolicy))
	if opts.SourceProjectPath == "" || opts.TargetProjectPath == "" || opts.StagingProjectPath == "" || opts.StagingDirectory == "" || opts.TargetDirectory == "" || opts.SourceProjectUUID == "" || opts.TargetProjectUUID == "" {
		return ExportManifestV2{}, errors.New("folder export requires complete source, staging and target identities")
	}
	if opts.MediaPolicy != MediaPolicyReferenceOnly && opts.MediaPolicy != MediaPolicyCopyReferenced {
		return ExportManifestV2{}, fmt.Errorf("unsupported media policy %q", opts.MediaPolicy)
	}
	if !pathWithin(opts.StagingProjectPath, opts.StagingDirectory) {
		return ExportManifestV2{}, errors.New("staging project path escapes staging directory")
	}
	if !samePath(filepath.Dir(opts.TargetProjectPath), opts.TargetDirectory) {
		return ExportManifestV2{}, errors.New("target project path must be directly inside target directory")
	}
	if _, err := os.Stat(opts.StagingProjectPath); err != nil {
		return ExportManifestV2{}, fmt.Errorf("staged project snapshot is missing: %w", err)
	}
	if _, err := os.Stat(opts.TargetDirectory); err == nil {
		return ExportManifestV2{}, errors.New("target folder already exists")
	} else if !os.IsNotExist(err) {
		return ExportManifestV2{}, err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(opts.StagingDirectory)
		}
	}()

	for _, root := range []string{
		filepath.Join(opts.StagingDirectory, projectstore.HistoryDirName, opts.TargetProjectUUID),
		filepath.Join(opts.StagingDirectory, projectstore.AgentDirName, opts.TargetProjectUUID),
		filepath.Join(opts.StagingDirectory, projectstore.DerivedDirName, opts.TargetProjectUUID),
	} {
		_, statErr := os.Stat(root)
		if statErr != nil && !os.IsNotExist(statErr) {
			return ExportManifestV2{}, statErr
		}
		if statErr == nil {
			if err := projectstore.RebindProjectTree(root, opts.StagingProjectPath, opts.TargetProjectUUID, opts.TargetProjectPath, opts.TargetProjectUUID); err != nil {
				return ExportManifestV2{}, err
			}
		}
	}

	mediaEntries := make([]ExportMediaEntry, 0, len(opts.Media))
	for _, source := range opts.Media {
		entry := ExportMediaEntry{
			Kind: "audio", SourcePath: cleanAbs(source.SourcePath), TargetPath: filepath.ToSlash(filepath.Clean(source.TargetPath)),
			Size: source.Size, Policy: opts.MediaPolicy,
		}
		if entry.SourcePath == "" {
			entry.Status = "missing"
			if opts.MediaPolicy == MediaPolicyReferenceOnly {
				mediaEntries = append(mediaEntries, entry)
				continue
			}
			return ExportManifestV2{}, errors.New("folder export media source path is missing")
		}
		info, statErr := os.Stat(entry.SourcePath)
		if statErr != nil || !info.Mode().IsRegular() {
			entry.Status = "missing"
			if opts.MediaPolicy == MediaPolicyReferenceOnly {
				entry.TargetPath = ""
				mediaEntries = append(mediaEntries, entry)
				continue
			}
			return ExportManifestV2{}, fmt.Errorf("referenced audio is missing: %s", entry.SourcePath)
		}
		entry.Size = info.Size()
		if opts.MediaPolicy == MediaPolicyReferenceOnly {
			entry.TargetPath = ""
			entry.Status = "referenced"
			mediaEntries = append(mediaEntries, entry)
			continue
		}
		if entry.TargetPath == "" || filepath.IsAbs(entry.TargetPath) {
			return ExportManifestV2{}, errors.New("copied media requires a relative target path")
		}
		target := filepath.Join(opts.StagingDirectory, filepath.FromSlash(entry.TargetPath))
		if !pathWithin(target, opts.StagingDirectory) {
			return ExportManifestV2{}, errors.New("media target escapes staging directory")
		}
		hash, size, err := copyFileHashed(entry.SourcePath, target)
		if err != nil {
			entry.Status = "copy_failed"
			return ExportManifestV2{}, err
		}
		entry.Size = size
		entry.SHA256 = hash
		entry.Status = "copied"
		mediaEntries = append(mediaEntries, entry)
	}

	originUUID := opts.SourceProjectUUID
	if sourceRoots, err := projectstore.Resolve(opts.SourceProjectPath, opts.SourceProjectUUID); err == nil {
		if sourceManifest, loadErr := projectstore.Load(sourceRoots); loadErr == nil && strings.TrimSpace(sourceManifest.OriginProjectUUID) != "" {
			originUUID = sourceManifest.OriginProjectUUID
		}
	}
	manifest := ExportManifestV2{
		SchemaVersion: ExportManifestSchemaV2, ProjectUUID: opts.TargetProjectUUID,
		SourceProjectUUID: opts.SourceProjectUUID, OriginProjectUUID: originUUID,
		ProjectFile: filepath.Base(opts.TargetProjectPath), ExportKind: "save_as_folder",
		MediaPolicy: opts.MediaPolicy, AudioSelfContained: opts.MediaPolicy == MediaPolicyCopyReferenced,
		UnbundledDependencyClasses: []string{"plugins", "sampler_libraries", "video", "external_ir"},
		PackageStatus:              "complete", CreatedAt: time.Now().UTC(), Media: mediaEntries,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return ExportManifestV2{}, err
	}
	if err := os.WriteFile(filepath.Join(opts.StagingDirectory, "package_manifest.v2.json"), append(data, '\n'), 0o600); err != nil {
		return ExportManifestV2{}, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.TargetDirectory), 0o755); err != nil {
		return ExportManifestV2{}, err
	}
	if err := os.Rename(opts.StagingDirectory, opts.TargetDirectory); err != nil {
		return ExportManifestV2{}, err
	}
	published = true
	return manifest, nil
}

func copyFileHashed(source, target string) (string, int64, error) {
	input, err := os.Open(source)
	if err != nil {
		return "", 0, err
	}
	defer input.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", 0, err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return "", 0, err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(output, hash), input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return "", 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return "", 0, closeErr
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func pathWithin(path, root string) bool {
	path = cleanAbs(path)
	root = cleanAbs(root)
	if path == "" || root == "" {
		return false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
