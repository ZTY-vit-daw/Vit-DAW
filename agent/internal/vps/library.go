package vps

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const libraryPathEnv = "VIT_VPS_LIBRARY_V3_PATH"

type libraryDocument struct {
	SchemaVersion string                 `json:"schema_version"`
	Inventory     []PluginInventoryEntry `json:"plugin_inventory"`
	Documents     []VPSDocument          `json:"documents"`
}

// Library persists the canonical user-level VPS Library. The Provider Catalog
// is deliberately rebuilt on read, rather than being written alongside it.
type Library struct {
	mu   sync.Mutex
	path string
	now  func() time.Time
}

func NewLibrary(path string) (*Library, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("VPS library path is required")
	}
	return &Library{path: path, now: func() time.Time { return time.Now().UTC() }}, nil
}

// OpenDefaultLibrary opens the user-level VPS Library without creating or
// mutating it. The file is created only by an explicit Upsert or Import call.
func OpenDefaultLibrary() (*Library, error) {
	return NewLibrary(DefaultLibraryPath())
}

// DefaultLibraryPath intentionally lives in the user configuration area. A
// project stores only a later Project Provider Instance / lease, never this
// reusable capability archive.
func DefaultLibraryPath() string {
	if path := strings.TrimSpace(os.Getenv(libraryPathEnv)); path != "" {
		return path
	}
	if root, err := os.UserConfigDir(); err == nil && strings.TrimSpace(root) != "" {
		return filepath.Join(root, "Vit", "Agent", "vps_library_v3.json")
	}
	return filepath.Join("VitApp", "Workspace", "State", "vps_library_v3.json")
}

func (l *Library) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

func (l *Library) List() ([]VPSDocument, error) {
	if l == nil {
		return nil, errors.New("VPS library is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	document, err := l.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]VPSDocument, 0, len(document.Documents))
	for _, item := range document.Documents {
		if err := item.Validate(); err != nil {
			return nil, fmt.Errorf("invalid persisted VPS %s: %w", item.ID, err)
		}
		out = append(out, cloneVPS(item))
	}
	sortVPS(out)
	return out, nil
}

func (l *Library) Get(id string) (VPSDocument, bool, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return VPSDocument{}, false, errors.New("VPS id is required")
	}
	documents, err := l.List()
	if err != nil {
		return VPSDocument{}, false, err
	}
	for _, document := range documents {
		if document.ID == id {
			return document, true, nil
		}
	}
	return VPSDocument{}, false, nil
}

// FindByPluginIdentity returns the one user-level VPS that represents the
// installed plugin family. Version and fingerprint changes intentionally still
// match the same record so a fresh learning pass can mark an old Credential
// stale instead of silently creating another dispatchable identity.
func (l *Library) FindByPluginIdentity(identity PluginIdentity) (VPSDocument, bool, error) {
	if err := identity.validDraft(); err != nil {
		return VPSDocument{}, false, err
	}
	documents, err := l.List()
	if err != nil {
		return VPSDocument{}, false, err
	}
	matches := []VPSDocument{}
	for _, document := range documents {
		if samePluginFamily(document.PluginIdentity, identity) {
			matches = append(matches, document)
		}
	}
	if len(matches) == 0 {
		return VPSDocument{}, false, nil
	}
	if len(matches) > 1 {
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.ID)
		}
		return VPSDocument{}, false, fmt.Errorf("plugin identity matches multiple VPS documents: %s", strings.Join(ids, ", "))
	}
	return matches[0], true, nil
}

func (l *Library) ListInventory() ([]PluginInventoryEntry, error) {
	if l == nil {
		return nil, errors.New("VPS library is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	document, err := l.loadLocked()
	if err != nil {
		return nil, err
	}
	out := make([]PluginInventoryEntry, 0, len(document.Inventory))
	for _, entry := range document.Inventory {
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("invalid persisted plugin inventory entry %s: %w", entry.ID, err)
		}
		out = append(out, cloneInventoryEntry(entry))
	}
	sortInventory(out)
	return out, nil
}

// UpsertInventory records discovery only. It cannot create a VPS, Credential
// or Catalog entry, which keeps Inventory and dispatch authority separate.
func (l *Library) UpsertInventory(input PluginInventoryEntry) (PluginInventoryEntry, error) {
	if l == nil {
		return PluginInventoryEntry{}, errors.New("VPS library is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	document, err := l.loadLocked()
	if err != nil {
		return PluginInventoryEntry{}, err
	}
	now := l.currentTime()
	if strings.TrimSpace(input.ID) == "" {
		input.ID = NewInventoryEntry(input.Identity, input.Source, now).ID
	}
	if input.FirstSeen.IsZero() {
		input.FirstSeen = now
	}
	if input.LastSeen.IsZero() {
		input.LastSeen = now
	}
	for index, existing := range document.Inventory {
		if existing.ID != input.ID {
			continue
		}
		input.FirstSeen = existing.FirstSeen
		input.LastSeen = now
		if err := input.Validate(); err != nil {
			return PluginInventoryEntry{}, err
		}
		document.Inventory[index] = cloneInventoryEntry(input)
		sortInventory(document.Inventory)
		if err := l.persistLocked(document); err != nil {
			return PluginInventoryEntry{}, err
		}
		return cloneInventoryEntry(input), nil
	}
	if err := input.Validate(); err != nil {
		return PluginInventoryEntry{}, err
	}
	document.Inventory = append(document.Inventory, cloneInventoryEntry(input))
	sortInventory(document.Inventory)
	if err := l.persistLocked(document); err != nil {
		return PluginInventoryEntry{}, err
	}
	return cloneInventoryEntry(input), nil
}

// Upsert is the explicit editing API. It advances the VPS revision and is
// suitable for a deliberate learning/conformance update. Migration callers
// should use ImportDraft so they can never overwrite a newer VPS implicitly.
func (l *Library) Upsert(input VPSDocument) (VPSDocument, error) {
	if l == nil {
		return VPSDocument{}, errors.New("VPS library is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	document, err := l.loadLocked()
	if err != nil {
		return VPSDocument{}, err
	}
	now := l.currentTime()
	if input.SchemaVersion != "" && input.SchemaVersion != DocumentSchemaVersion {
		return VPSDocument{}, fmt.Errorf("cannot upsert non-v3 VPS schema %q; use an explicit migration entry point", input.SchemaVersion)
	}
	input.SchemaVersion = DocumentSchemaVersion
	if strings.TrimSpace(input.ID) == "" {
		input.ID = NewDocumentID(input.PluginIdentity)
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	if input.UpdatedAt.IsZero() {
		input.UpdatedAt = now
	}

	found := false
	for index, existing := range document.Documents {
		if existing.ID != input.ID {
			continue
		}
		found = true
		if err := validateDocumentTransitions(existing, input); err != nil {
			return VPSDocument{}, err
		}
		input.CreatedAt = existing.CreatedAt
		input.UpdatedAt = now
		input.Revision = existing.Revision + 1
		if err := input.Validate(); err != nil {
			return VPSDocument{}, err
		}
		document.Documents[index] = cloneVPS(input)
		break
	}
	if !found {
		if input.Revision < 1 {
			input.Revision = 1
		}
		input.UpdatedAt = now
		if err := input.Validate(); err != nil {
			return VPSDocument{}, err
		}
		document.Documents = append(document.Documents, cloneVPS(input))
	}
	sortVPS(document.Documents)
	if err := l.persistLocked(document); err != nil {
		return VPSDocument{}, err
	}
	return cloneVPS(input), nil
}

func validateDocumentTransitions(previous, next VPSDocument) error {
	previousCredentials := make(map[string]ProviderCredential, len(previous.ProviderCredentials))
	for _, credential := range previous.ProviderCredentials {
		previousCredentials[credential.ID] = credential
	}
	nextCredentials := make(map[string]ProviderCredential, len(next.ProviderCredentials))
	for _, credential := range next.ProviderCredentials {
		nextCredentials[credential.ID] = credential
		if old, ok := previousCredentials[credential.ID]; ok {
			if err := validateCredentialTransition(old, credential); err != nil {
				return err
			}
		}
	}
	for id, credential := range previousCredentials {
		if _, retained := nextCredentials[id]; !retained && credential.Dispatchable() {
			return fmt.Errorf("cannot remove dispatchable credential %s; revoke it instead", id)
		}
	}
	return nil
}

// ImportDraft persists a legacy/imported VPS only if there is no document with
// that migration ID already. It never replaces an existing archive and never
// promotes a draft to a Credential.
func (l *Library) ImportDraft(input VPSDocument) (VPSDocument, bool, error) {
	if l == nil {
		return VPSDocument{}, false, errors.New("VPS library is nil")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	document, err := l.loadLocked()
	if err != nil {
		return VPSDocument{}, false, err
	}
	if input.SchemaVersion != "" && input.SchemaVersion != DocumentSchemaVersion {
		return VPSDocument{}, false, fmt.Errorf("cannot import non-v3 VPS schema %q without migration", input.SchemaVersion)
	}
	for _, existing := range document.Documents {
		if existing.ID == input.ID {
			return cloneVPS(existing), false, nil
		}
	}
	for _, credential := range input.ProviderCredentials {
		if credential.Status == CredentialVerified {
			return VPSDocument{}, false, fmt.Errorf("legacy import %s cannot introduce a verified credential", input.ID)
		}
	}
	now := l.currentTime()
	input.SchemaVersion = DocumentSchemaVersion
	if strings.TrimSpace(input.ID) == "" {
		input.ID = NewDocumentID(input.PluginIdentity)
	}
	if input.Revision < 1 {
		input.Revision = 1
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if err := input.Validate(); err != nil {
		return VPSDocument{}, false, err
	}
	document.Documents = append(document.Documents, cloneVPS(input))
	sortVPS(document.Documents)
	if err := l.persistLocked(document); err != nil {
		return VPSDocument{}, false, err
	}
	return cloneVPS(input), true, nil
}

// Catalog derives an up-to-date, read-only Provider Catalog from the library.
func (l *Library) Catalog() (ProviderCatalog, error) {
	documents, err := l.List()
	if err != nil {
		return ProviderCatalog{}, err
	}
	return DeriveProviderCatalog(documents, l.currentTime())
}

func (l *Library) loadLocked() (libraryDocument, error) {
	data, err := os.ReadFile(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return libraryDocument{SchemaVersion: LibrarySchemaVersion, Inventory: []PluginInventoryEntry{}, Documents: []VPSDocument{}}, nil
	}
	if err != nil {
		return libraryDocument{}, fmt.Errorf("read VPS library: %w", err)
	}
	var document libraryDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return libraryDocument{}, fmt.Errorf("decode VPS library: %w", err)
	}
	if document.SchemaVersion == "" {
		document.SchemaVersion = LibrarySchemaVersion
	}
	if document.SchemaVersion != LibrarySchemaVersion {
		return libraryDocument{}, fmt.Errorf("unsupported VPS library schema %q", document.SchemaVersion)
	}
	if document.Documents == nil {
		document.Documents = []VPSDocument{}
	}
	if document.Inventory == nil {
		document.Inventory = []PluginInventoryEntry{}
	}
	return document, nil
}

func (l *Library) persistLocked(document libraryDocument) error {
	document.SchemaVersion = LibrarySchemaVersion
	sortInventory(document.Inventory)
	sortVPS(document.Documents)
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode VPS library: %w", err)
	}
	directory := filepath.Dir(l.path)
	if directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create VPS library directory: %w", err)
		}
	}
	temporary := l.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write VPS library: %w", err)
	}
	if err := os.Rename(temporary, l.path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("commit VPS library: %w", err)
	}
	return nil
}

func (l *Library) currentTime() time.Time {
	if l != nil && l.now != nil {
		return l.now().UTC()
	}
	return time.Now().UTC()
}

func cloneVPS(input VPSDocument) VPSDocument {
	data, err := json.Marshal(input)
	if err != nil {
		return input
	}
	var out VPSDocument
	if err := json.Unmarshal(data, &out); err != nil {
		return input
	}
	return out
}

func sortVPS(documents []VPSDocument) {
	sort.Slice(documents, func(i, j int) bool { return documents[i].ID < documents[j].ID })
}

func cloneInventoryEntry(input PluginInventoryEntry) PluginInventoryEntry {
	data, err := json.Marshal(input)
	if err != nil {
		return input
	}
	var out PluginInventoryEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return input
	}
	return out
}

func sortInventory(entries []PluginInventoryEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
}

func samePluginFamily(left, right PluginIdentity) bool {
	equal := func(a, b string) bool {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	if strings.TrimSpace(left.ProfileKey) != "" && strings.TrimSpace(right.ProfileKey) != "" {
		return equal(left.ProfileKey, right.ProfileKey)
	}
	if !equal(left.Manufacturer, right.Manufacturer) || !equal(left.Name, right.Name) || !equal(left.Format, right.Format) {
		return false
	}
	leftPath := strings.TrimSpace(left.InstallPath)
	rightPath := strings.TrimSpace(right.InstallPath)
	return leftPath == "" || rightPath == "" || equal(leftPath, rightPath)
}
