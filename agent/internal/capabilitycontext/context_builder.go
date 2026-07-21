package capabilitycontext

import "strings"

type ContextInput struct {
	Manifest ContextManifest
	Sources  map[string]any
	Budget   Budget
}

type ContextBuildResult struct {
	SchemaVersion  string                         `json:"schema_version"`
	BuilderVersion string                         `json:"builder_version"`
	ManifestID     string                         `json:"manifest_id"`
	CapabilityID   string                         `json:"capability_id"`
	Sources        map[string]ContextSourceResult `json:"sources,omitempty"`
	RowSets        map[string]ContextRowSet       `json:"row_sets,omitempty"`
	Merges         map[string]ContextRowSet       `json:"merges,omitempty"`
	FieldSpecs     map[string]ContextFieldSpec    `json:"field_specs,omitempty"`
}

type ContextSourceResult struct {
	ID      string         `json:"id"`
	Status  string         `json:"status"`
	Reason  string         `json:"reason,omitempty"`
	Payload map[string]any `json:"-"`
}

type ContextRowSet struct {
	ID          string       `json:"id"`
	SourceID    string       `json:"source_id,omitempty"`
	Status      string       `json:"status"`
	KnownCount  int          `json:"known_count,omitempty"`
	TotalCount  int          `json:"total_count,omitempty"`
	Reason      string       `json:"reason,omitempty"`
	EvidenceRef string       `json:"evidence_ref,omitempty"`
	Rows        []ContextRow `json:"rows,omitempty"`
}

type ContextRow struct {
	Key          string         `json:"key,omitempty"`
	Data         map[string]any `json:"data,omitempty"`
	EvidenceRefs []string       `json:"evidence_refs,omitempty"`
	SourceRows   []string       `json:"source_rows,omitempty"`
}

func BuildCapabilityContext(in ContextInput) ContextBuildResult {
	manifest := normalizeContextManifest(in.Manifest)
	result := ContextBuildResult{
		SchemaVersion:  ContextManifestSchemaVersion,
		BuilderVersion: CapabilityContextBuilderVersion,
		ManifestID:     manifest.ManifestID,
		CapabilityID:   manifest.CapabilityID,
		Sources:        map[string]ContextSourceResult{},
		RowSets:        map[string]ContextRowSet{},
		Merges:         map[string]ContextRowSet{},
		FieldSpecs:     map[string]ContextFieldSpec{},
	}
	for _, field := range manifest.Fields {
		if strings.TrimSpace(field.ID) != "" {
			result.FieldSpecs[field.ID] = field
		}
	}
	for _, spec := range manifest.Sources {
		source := mapValue(in.Sources[spec.ID])
		status := "ready"
		reason := ""
		if len(source) == 0 {
			status = "missing"
			reason = "source_payload_missing"
			if spec.Required {
				reason = "required_source_payload_missing"
			}
		}
		result.Sources[spec.ID] = ContextSourceResult{
			ID:      spec.ID,
			Status:  status,
			Reason:  reason,
			Payload: source,
		}
	}
	for _, spec := range manifest.RowSets {
		result.RowSets[spec.ID] = buildContextRowSet(result.Sources[spec.SourceID], spec, manifest.Fields)
	}
	for _, spec := range manifest.Merges {
		result.Merges[spec.ID] = mergeContextRowSets(result.RowSets, spec, manifest.Fields)
	}
	return result
}

func (r ContextBuildResult) SourceMap(id string) map[string]any {
	if r.Sources == nil {
		return nil
	}
	return cloneMap(r.Sources[id].Payload)
}

func (r ContextBuildResult) Rows(id string) []ContextRow {
	if rowSet, ok := r.Merges[id]; ok {
		return append([]ContextRow(nil), rowSet.Rows...)
	}
	if rowSet, ok := r.RowSets[id]; ok {
		return append([]ContextRow(nil), rowSet.Rows...)
	}
	return nil
}

func (r ContextBuildResult) EvidenceRefs() []string {
	refs := []string{}
	for _, set := range r.RowSets {
		if set.KnownCount > 0 && set.EvidenceRef != "" {
			refs = addUnique(refs, set.EvidenceRef)
		}
	}
	return refs
}

func buildContextRowSet(source ContextSourceResult, spec ContextRowSetSpec, fields []ContextFieldSpec) ContextRowSet {
	out := ContextRowSet{
		ID:          spec.ID,
		SourceID:    spec.SourceID,
		Status:      "missing",
		Reason:      "row_source_missing",
		EvidenceRef: spec.EvidenceRef,
	}
	if len(source.Payload) == 0 {
		if source.Reason != "" {
			out.Reason = source.Reason
		}
		return out
	}
	for _, path := range spec.Paths {
		rows := rowsValue(valueAtPath(source.Payload, path))
		if len(rows) == 0 {
			continue
		}
		out.Rows = append(out.Rows, contextRowsFromMaps(rows, spec.KeyAliases, spec.EvidenceRef, spec.ID, fields)...)
	}
	out.KnownCount = len(out.Rows)
	out.TotalCount = len(out.Rows)
	if len(out.Rows) > 0 {
		out.Status = "ready"
		out.Reason = ""
	}
	return out
}

func contextRowsFromMaps(rows []map[string]any, keyAliases []string, evidenceRef string, sourceID string, fields []ContextFieldSpec) []ContextRow {
	out := make([]ContextRow, 0, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		data := normalizeContextRowFields(row, fields)
		key := contextRowKey(data, keyAliases)
		if key == "" {
			key = sourceID + "#" + cleanText(i)
		}
		out = append(out, ContextRow{
			Key:          key,
			Data:         data,
			EvidenceRefs: addUnique(nil, evidenceRef),
			SourceRows:   []string{sourceID},
		})
	}
	return out
}

func mergeContextRowSets(rowSets map[string]ContextRowSet, spec ContextMergeSpec, fields []ContextFieldSpec) ContextRowSet {
	out := ContextRowSet{
		ID:     spec.ID,
		Status: "missing",
		Reason: "merge_rows_missing",
	}
	byKey := map[string]int{}
	for _, rowSetID := range spec.RowSetIDs {
		rowSet := rowSets[rowSetID]
		if rowSet.EvidenceRef != "" && out.EvidenceRef == "" {
			out.EvidenceRef = rowSet.EvidenceRef
		}
		for _, row := range rowSet.Rows {
			if len(row.Data) == 0 {
				continue
			}
			key := row.Key
			if key == "" {
				key = contextRowKey(row.Data, spec.KeyAliases)
			}
			if key == "" {
				key = rowSetID + "#" + cleanText(len(out.Rows))
			}
			if existingIndex, ok := byKey[key]; ok {
				mergeContextRowInto(&out.Rows[existingIndex], row, spec.MergePolicy, fields)
				continue
			}
			next := ContextRow{
				Key:          key,
				Data:         normalizeContextRowFields(row.Data, fields),
				EvidenceRefs: addUnique(nil, row.EvidenceRefs...),
				SourceRows:   append([]string(nil), row.SourceRows...),
			}
			if len(next.SourceRows) == 0 {
				next.SourceRows = []string{rowSetID}
			}
			byKey[key] = len(out.Rows)
			out.Rows = append(out.Rows, next)
		}
	}
	out.KnownCount = len(out.Rows)
	out.TotalCount = len(out.Rows)
	if len(out.Rows) > 0 {
		out.Status = "ready"
		out.Reason = ""
	}
	return out
}

func mergeContextRowInto(dst *ContextRow, src ContextRow, policy string, fields []ContextFieldSpec) {
	if dst == nil {
		return
	}
	if dst.Data == nil {
		dst.Data = map[string]any{}
	}
	srcData := normalizeContextRowFields(src.Data, fields)
	for key, value := range srcData {
		switch strings.ToLower(strings.TrimSpace(policy)) {
		case "override":
			if cleanText(value) != "" {
				dst.Data[key] = value
			}
		default:
			if _, exists := dst.Data[key]; !exists || dst.Data[key] == nil || cleanText(dst.Data[key]) == "" {
				dst.Data[key] = value
			}
		}
	}
	dst.Data = normalizeContextRowFields(dst.Data, fields)
	dst.EvidenceRefs = addUnique(dst.EvidenceRefs, src.EvidenceRefs...)
	dst.SourceRows = addUnique(dst.SourceRows, src.SourceRows...)
}

func normalizeContextRowFields(row map[string]any, fields []ContextFieldSpec) map[string]any {
	out := cloneMap(row)
	if out == nil {
		out = map[string]any{}
	}
	for _, field := range fields {
		fieldID := strings.TrimSpace(field.ID)
		if fieldID == "" || cleanText(out[fieldID]) != "" {
			continue
		}
		for _, alias := range field.Aliases {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if value, ok := out[alias]; ok && cleanText(value) != "" {
				out[fieldID] = value
				break
			}
		}
	}
	return out
}

func contextRowKey(row map[string]any, aliases []string) string {
	for _, alias := range aliases {
		if text := firstText(row, alias); text != "" {
			return text
		}
	}
	return ""
}

func valueAtPath(root map[string]any, path []string) any {
	var current any = root
	for _, part := range path {
		row := mapValue(current)
		if len(row) == 0 {
			return nil
		}
		current = row[part]
	}
	return current
}
