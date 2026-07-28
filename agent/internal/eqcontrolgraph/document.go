package eqcontrolgraph

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SchemaVersion       = "vit.eq_vps.control_graph.v1"
	AttestationVersion  = "vit.eq_vps.verification.v1"
	MaxDocumentBytes    = 8 << 20
	MaxAttestationBytes = 1 << 20
)

type Document struct {
	SchemaVersion        string                 `json:"schema_version"`
	Plugin               PluginIdentity         `json:"plugin"`
	SurfaceSignature     string                 `json:"surface_signature"`
	PublicClassification string                 `json:"public_classification"`
	ChannelContract      string                 `json:"channel_contract"`
	Bindings             map[string]Binding     `json:"bindings"`
	Sections             []Section              `json:"sections"`
	Provenance           map[string][]string    `json:"provenance,omitempty"`
	Unresolved           []string               `json:"unresolved,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

type PluginIdentity struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Name         string `json:"name"`
	Format       string `json:"format"`
	Version      string `json:"version,omitempty"`
}

type Binding struct {
	Kind          string                 `json:"kind"`
	ParameterID   string                 `json:"parameter_id"`
	ParameterName string                 `json:"parameter_name"`
	Channel       string                 `json:"channel"`
	Domain        *NumericDomain         `json:"domain,omitempty"`
	Values        map[string]EnumValue   `json:"values,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type NumericDomain struct {
	Unit     string       `json:"unit"`
	Minimum  float64      `json:"minimum"`
	Maximum  float64      `json:"maximum"`
	Curve    [][2]float64 `json:"curve"`
	ValueLaw string       `json:"value_law,omitempty"`
}

type EnumValue struct {
	Normalized float64 `json:"normalized"`
	Label      string  `json:"label"`
}

type Section struct {
	SectionKey  string                 `json:"section_key"`
	DisplayName string                 `json:"display_name,omitempty"`
	Addressing  string                 `json:"addressing"`
	Activation  Activation             `json:"activation"`
	Selection   Selection              `json:"selection"`
	Bindings    map[string]string      `json:"bindings"`
	Shapes      map[string]Shape       `json:"shapes"`
	Programs    map[string]Program     `json:"programs"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type Activation struct {
	State string `json:"state"`
}

type Selection struct {
	FrequencyBinding string   `json:"frequency_binding,omitempty"`
	FixedFrequencyHz *float64 `json:"fixed_frequency_hz,omitempty"`
}

type Shape struct {
	Actions map[string]ActionContract `json:"actions"`
}

type ActionContract struct {
	Program        string   `json:"program"`
	RequiredInputs []string `json:"required_inputs,omitempty"`
	OptionalInputs []string `json:"optional_inputs,omitempty"`
}

type Program struct {
	Inputs map[string]InputSpec `json:"inputs,omitempty"`
	Writes []ProgramWrite       `json:"writes"`
}

type InputSpec struct {
	Type string `json:"type"`
}

type ProgramWrite struct {
	Binding string      `json:"binding"`
	Role    string      `json:"role"`
	Phase   string      `json:"phase,omitempty"`
	When    *Expression `json:"when,omitempty"`
	Value   Expression  `json:"value"`
}

type Expression struct {
	Op        string                `json:"op"`
	Input     string                `json:"input,omitempty"`
	Value     interface{}           `json:"value,omitempty"`
	Args      []Expression          `json:"args,omitempty"`
	Condition *Expression           `json:"condition,omitempty"`
	Then      *Expression           `json:"then,omitempty"`
	Else      *Expression           `json:"else,omitempty"`
	Key       *Expression           `json:"key,omitempty"`
	Cases     map[string]Expression `json:"cases,omitempty"`
	Default   *Expression           `json:"default,omitempty"`
}

type Attestation struct {
	SchemaVersion    string            `json:"schema_version"`
	VPSHash          string            `json:"vps_hash"`
	SurfaceSignature string            `json:"surface_signature"`
	Status           string            `json:"status"`
	VerifiedAt       time.Time         `json:"verified_at"`
	StaticChecks     int               `json:"static_checks"`
	LiveChecks       LiveCheckEvidence `json:"live_checks"`
	Evidence         []string          `json:"evidence,omitempty"`
}

type LiveCheckEvidence struct {
	ApplyReadback          bool `json:"apply_readback"`
	FormalUndoZeroDrift    bool `json:"formal_undo_zero_drift"`
	UnloadRestoresBaseline bool `json:"unload_restores_baseline"`
}

func LoadDocument(path string) (Document, error) {
	raw, err := readBoundedFile(path, MaxDocumentBytes)
	if err != nil {
		return Document{}, err
	}
	var doc Document
	if err = decodeStrict(raw, &doc); err != nil {
		return Document{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err = doc.Validate(); err != nil {
		return Document{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return doc, nil
}

func SaveDocument(path string, doc Document) error {
	if err := doc.Validate(); err != nil {
		return err
	}
	return saveJSON(path, doc)
}

func LoadAttestation(path string) (Attestation, error) {
	raw, err := readBoundedFile(path, MaxAttestationBytes)
	if err != nil {
		return Attestation{}, err
	}
	var attestation Attestation
	if err = decodeStrict(raw, &attestation); err != nil {
		return Attestation{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return attestation, nil
}

func readBoundedFile(path string, maximum int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maximum {
		return nil, fmt.Errorf("%s exceeds maximum size %d bytes", path, maximum)
	}
	return os.ReadFile(path)
}

func SaveAttestation(path string, attestation Attestation) error {
	return saveJSON(path, attestation)
}

func HashDocument(doc Document) (string, error) {
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func AttestationPath(documentPath string) string {
	lower := strings.ToLower(documentPath)
	if strings.HasSuffix(lower, ".vps.json") {
		return documentPath[:len(documentPath)-len(".vps.json")] + ".verification.json"
	}
	return documentPath + ".verification.json"
}

func decodeStrict(raw []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func saveJSON(path string, value interface{}) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err = os.WriteFile(temporary, raw, 0600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
