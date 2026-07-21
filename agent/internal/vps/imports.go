package vps

import (
	"fmt"
	"time"

	"vit-daw-agent/internal/spallab"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

// ImportResult describes one explicit, non-destructive migration operation.
// Imported is false when a matching migration document already exists.
type ImportResult struct {
	VPS      VPSDocument `json:"vps"`
	Imported bool        `json:"imported"`
	Source   string      `json:"source"`
}

func (l *Library) ImportPluginSkillDocumentV2(skill plugingrabber.PluginSkillDocument, now time.Time) (ImportResult, error) {
	document, err := MigratePluginSkillDocumentV2(skill, now)
	if err != nil {
		return ImportResult{}, err
	}
	stored, imported, err := l.ImportDraft(document)
	return ImportResult{VPS: stored, Imported: imported, Source: migrationSourcePluginSkillV2}, err
}

func (l *Library) ImportProfilePatch(identity PluginIdentity, patch plugingrabber.ProfilePatch, now time.Time) (ImportResult, error) {
	document, err := MigrateProfilePatch(identity, patch, now)
	if err != nil {
		return ImportResult{}, err
	}
	stored, imported, err := l.ImportDraft(document)
	return ImportResult{VPS: stored, Imported: imported, Source: migrationSourceProfilePatch}, err
}

func (l *Library) ImportLegacyVPS(data []byte, now time.Time) (ImportResult, error) {
	document, err := MigrateLegacyVPS(data, now)
	if err != nil {
		return ImportResult{}, err
	}
	stored, imported, err := l.ImportDraft(document)
	return ImportResult{VPS: stored, Imported: imported, Source: migrationSourceLegacyVPS}, err
}

func (l *Library) ImportSPALV0ProviderRecord(record spallab.ProviderRecord, now time.Time) (ImportResult, error) {
	document, err := MigrateSPALV0ProviderRecord(record, now)
	if err != nil {
		return ImportResult{}, err
	}
	stored, imported, err := l.ImportDraft(document)
	return ImportResult{VPS: stored, Imported: imported, Source: migrationSourceSPALV0}, err
}

// ImportSPALV0ProviderStore is a read-only compatibility bridge from the
// existing v0 provider-store. It never writes to that store and cannot make a
// v0 record dispatchable in the v3 Catalog.
func (l *Library) ImportSPALV0ProviderStore(store *spallab.ProviderStore, now time.Time) ([]ImportResult, error) {
	if store == nil {
		return nil, fmt.Errorf("SPAL v0 provider store is required")
	}
	records, err := store.List()
	if err != nil {
		return nil, fmt.Errorf("list SPAL v0 provider records: %w", err)
	}
	results := make([]ImportResult, 0, len(records))
	for _, record := range records {
		result, err := l.ImportSPALV0ProviderRecord(record, now)
		if err != nil {
			return nil, fmt.Errorf("import SPAL v0 provider record %s: %w", record.ID, err)
		}
		results = append(results, result)
	}
	return results, nil
}

// ImportDefaultSPALV0ReferenceEQProviderStore is the explicit bridge for the
// existing product-facing v0 Reference EQ store. It is intentionally opt-in;
// opening a v3 Library never mutates or silently imports user records.
func (l *Library) ImportDefaultSPALV0ReferenceEQProviderStore(now time.Time) ([]ImportResult, error) {
	store, err := spallab.NewProviderStore(spallab.DefaultReferenceEQProviderStorePath())
	if err != nil {
		return nil, fmt.Errorf("open default SPAL v0 Reference EQ provider store: %w", err)
	}
	return l.ImportSPALV0ProviderStore(store, now)
}
