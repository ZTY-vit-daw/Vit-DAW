package artifacts

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/projectstore"
)

const (
	DefaultDirName   = "Artifacts"
	ManifestFile     = "manifest.jsonl"
	DefaultTextLimit = 12000
)

type Store struct {
	Root string
	Now  func() time.Time
}

func DefaultRoot() string {
	if roots, ok := projectstore.Current(); ok {
		return filepath.Join(roots.Agent, "artifacts")
	}
	if devRoot := strings.TrimSpace(os.Getenv("VIT_DAW_DEV_ROOT")); devRoot != "" {
		root := filepath.Clean(devRoot)
		if _, err := os.Stat(filepath.Join(root, "VitApp", "Workspace")); err == nil {
			return filepath.Join(root, "VitApp", "Workspace", DefaultDirName)
		}
	}
	return filepath.Join(os.TempDir(), "vit-daw-unbound", fmt.Sprint(os.Getpid()), "artifacts")
}

func NewStore(root string) Store {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot()
	}
	return Store{Root: filepath.Clean(root), Now: time.Now}
}

func (s Store) Upsert(a Artifact) (Artifact, error) {
	if strings.TrimSpace(a.ID) == "" {
		a.ID = "art_" + randomID()
	}
	if strings.TrimSpace(a.Status) == "" {
		a.Status = "ready"
	}
	if strings.TrimSpace(a.CreatedAt) == "" {
		now := time.Now
		if s.Now != nil {
			now = s.Now
		}
		a.CreatedAt = now().UTC().Format(time.RFC3339Nano)
	}
	if a.Metadata == nil {
		a.Metadata = map[string]any{}
	}
	if err := os.MkdirAll(s.Root, 0755); err != nil {
		return Artifact{}, err
	}
	f, err := os.OpenFile(filepath.Join(s.Root, ManifestFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return Artifact{}, err
	}
	defer f.Close()
	line, err := json.Marshal(a)
	if err != nil {
		return Artifact{}, err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return Artifact{}, err
	}
	return a, nil
}

func (s Store) Get(id string) (Artifact, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Artifact{}, errors.New("artifact id is required")
	}
	items, err := s.List()
	if err != nil {
		return Artifact{}, err
	}
	for _, a := range items {
		if a.ID == id {
			if strings.EqualFold(strings.TrimSpace(a.Status), "deleted") {
				return Artifact{}, os.ErrNotExist
			}
			return a, nil
		}
	}
	return Artifact{}, os.ErrNotExist
}

func (s Store) List() ([]Artifact, error) {
	path := filepath.Join(s.Root, ManifestFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	latest := map[string]Artifact{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var a Artifact
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			continue
		}
		if strings.TrimSpace(a.ID) != "" {
			latest[a.ID] = a
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	out := make([]Artifact, 0, len(latest))
	for _, a := range latest {
		if strings.EqualFold(strings.TrimSpace(a.Status), "deleted") {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}

func (s Store) Rename(id, title string) (Artifact, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Artifact{}, errors.New("artifact title is required")
	}
	a, err := s.Get(id)
	if err != nil {
		return Artifact{}, err
	}
	a.Title = title
	if strings.TrimSpace(a.Status) == "" {
		a.Status = "ready"
	}
	return s.Upsert(a)
}

func (s Store) Delete(id string) (Artifact, error) {
	a, err := s.Get(id)
	if err != nil {
		return Artifact{}, err
	}
	a.Status = "deleted"
	return s.Upsert(a)
}

func Summaries(items []Artifact) []Summary {
	out := make([]Summary, 0, len(items))
	for _, a := range items {
		out = append(out, a.CompactSummary())
	}
	return out
}

func randomID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format("150405.000000")))
	}
	return hex.EncodeToString(b[:])
}
