package plugingrabber

type LearningTarget struct {
	TrackID    string
	PluginID   string
	PluginName string
	Intent     string
}

type LearningMode string

const (
	LearningModeAutoLearn LearningMode = "auto_learn"
	LearningModeTeach     LearningMode = "teach"
)

type LoadTarget struct {
	TrackID     string
	PluginQuery string
	PluginPath  string
	PluginName  string
	Intent      string
	IntentKind  string
}

type LoadCandidate struct {
	Name         string
	Path         string
	Format       string
	Manufacturer string
	Category     string
	Score        int
	IsInstrument bool
}

type ParameterDigest struct {
	TrackID              string                 `json:"track_id"`
	PluginID             string                 `json:"plugin_id"`
	PluginName           string                 `json:"plugin_name,omitempty"`
	PluginIdentity       map[string]any         `json:"plugin_identity,omitempty"`
	TemplateRole         string                 `json:"template_role,omitempty"`
	ProfileSource        string                 `json:"profile_source,omitempty"`
	ProfileApplied       bool                   `json:"profile_applied"`
	ProfileStaleParamIDs []string               `json:"profile_stale_param_ids,omitempty"`
	GlobalProfileApplied bool                   `json:"global_profile_applied,omitempty"`
	GlobalProfileSource  string                 `json:"global_profile_source,omitempty"`
	GlobalProfile        map[string]any         `json:"global_profile,omitempty"`
	PluginClass          string                 `json:"plugin_class,omitempty"`
	PluginGroups         []map[string]any       `json:"plugin_groups,omitempty"`
	VirtualControls      []map[string]any       `json:"virtual_controls,omitempty"`
	SafetyLimits         map[string]any         `json:"safety_limits,omitempty"`
	PluginSkill          map[string]any         `json:"plugin_skill,omitempty"`
	ParameterCount       int                    `json:"parameter_count"`
	QuickControls        []QuickControlDigest   `json:"quick_controls,omitempty"`
	RecommendedGroups    []RecommendedGroupInfo `json:"recommended_groups,omitempty"`
	Parameters           []ParameterInfo        `json:"parameters"`
}

type ParameterInfo struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name,omitempty"`
	RawName                string                 `json:"raw_param_name,omitempty"`
	Alias                  string                 `json:"alias,omitempty"`
	DisplayGroup           string                 `json:"display_group,omitempty"`
	NormalizedRole         string                 `json:"normalized_role,omitempty"`
	ControlRelevance       string                 `json:"control_relevance,omitempty"`
	HostControllable       bool                   `json:"host_controllable,omitempty"`
	IsBoolean              bool                   `json:"is_boolean,omitempty"`
	IsDiscrete             bool                   `json:"is_discrete,omitempty"`
	NumSteps               int                    `json:"num_steps,omitempty"`
	Unit                   string                 `json:"unit,omitempty"`
	Value                  any                    `json:"value,omitempty"`
	NormalizedValue        any                    `json:"normalized_value,omitempty"`
	ValueText              string                 `json:"value_text,omitempty"`
	Min                    any                    `json:"min,omitempty"`
	Max                    any                    `json:"max,omitempty"`
	DisplayProbe           *ParameterDisplayProbe `json:"display_probe,omitempty"`
	DisplayDomainCandidate *PluginDisplayDomain   `json:"display_domain_candidate,omitempty"`
}

type ParameterDisplayProbe struct {
	Mode           string                        `json:"mode,omitempty"`
	CurrentText    string                        `json:"current_text,omitempty"`
	Label          string                        `json:"label,omitempty"`
	Samples        []ParameterDisplayProbeSample `json:"samples,omitempty"`
	DiscreteLabels []ParameterDisplayProbeLabel  `json:"discrete_labels,omitempty"`
	AllLabels      []string                      `json:"all_labels,omitempty"`
	Capabilities   []string                      `json:"capabilities,omitempty"`
	Issues         []string                      `json:"issues,omitempty"`
}

type ParameterDisplayProbeSample struct {
	NormalizedValue float64 `json:"normalized_value"`
	Value           any     `json:"value,omitempty"`
	Text            string  `json:"text,omitempty"`
}

type ParameterDisplayProbeLabel struct {
	Index int    `json:"index"`
	Value any    `json:"value,omitempty"`
	Label string `json:"label,omitempty"`
}

type QuickControlDigest struct {
	ParamID        string `json:"param_id"`
	Label          string `json:"label,omitempty"`
	Widget         string `json:"widget,omitempty"`
	DisplayGroup   string `json:"display_group,omitempty"`
	NormalizedRole string `json:"normalized_role,omitempty"`
}

type RecommendedGroupInfo struct {
	Name           string   `json:"name"`
	ParameterCount int      `json:"parameter_count"`
	SampleIDs      []string `json:"sample_ids,omitempty"`
}

type ProfilePatch struct {
	Reply           string            `json:"reply,omitempty"`
	QuickControlIDs []string          `json:"quick_control_ids,omitempty"`
	Aliases         map[string]string `json:"aliases,omitempty"`
	DisplayGroups   map[string]string `json:"display_groups,omitempty"`
	NormalizedRoles map[string]string `json:"normalized_roles,omitempty"`

	// Layer 2: Extended profile fields (filled by LLM in Layer 3a)
	Class           string               `json:"class,omitempty"`
	Groups          []map[string]any     `json:"groups,omitempty"`
	VirtualControls []map[string]any     `json:"virtual_controls,omitempty"`
	Safety          map[string]any       `json:"safety,omitempty"`
	TeachModeRows   []TeachModeRowRecord `json:"teach_mode_rows,omitempty"`
}

const PluginSkillSchemaVersion = 2

const (
	PluginSkillStatusActive     = "active"
	PluginSkillStatusCandidate  = "candidate"
	PluginSkillStatusUnresolved = "unresolved"
)

type PluginSkillDocument struct {
	SchemaVersion int                     `json:"schema_version"`
	Identity      PluginSkillIdentity     `json:"identity"`
	Capabilities  PluginSkillCapabilities `json:"capabilities"`
	Components    []PluginSkillComponent  `json:"components"`
	Operations    []PluginSkillOperation  `json:"operations"`
	Safety        map[string]any          `json:"safety"`
	Preferences   PluginSkillPreferences  `json:"preferences"`
	Legacy        PluginSkillLegacy       `json:"legacy"`
}

type PluginSkillIdentity struct {
	Name               string `json:"name,omitempty"`
	Manufacturer       string `json:"manufacturer,omitempty"`
	Format             string `json:"format,omitempty"`
	Version            string `json:"version,omitempty"`
	Path               string `json:"path,omitempty"`
	ProfileKey         string `json:"profile_key,omitempty"`
	PluginID           string `json:"plugin_id,omitempty"`
	ParamSignatureHash string `json:"param_signature_hash,omitempty"`
	PrimaryClass       string `json:"primary_class,omitempty"`
}

type PluginSkillCapabilities struct {
	Types []string        `json:"types,omitempty"`
	Flags map[string]bool `json:"flags,omitempty"`
}

type PluginSkillComponent struct {
	ID     string                         `json:"id"`
	Role   string                         `json:"role,omitempty"`
	Label  string                         `json:"label,omitempty"`
	Params map[string]PluginSkillParamMap `json:"params,omitempty"`
}

type PluginSkillParamMap struct {
	ParamID           string               `json:"param_id"`
	Label             string               `json:"label,omitempty"`
	Source            string               `json:"source"`
	Confidence        float64              `json:"confidence"`
	Confirmed         bool                 `json:"confirmed"`
	Status            string               `json:"status,omitempty"`
	Locked            bool                 `json:"locked"`
	DisplayDomain     *PluginDisplayDomain `json:"display_domain,omitempty"`
	DisplayDomainText string               `json:"display_domain_text,omitempty"`
	Evidence          []PluginEvidence     `json:"evidence,omitempty"`
	Provenance        []PluginProvenance   `json:"provenance,omitempty"`
	MissingEvidence   []string             `json:"missing_evidence,omitempty"`
	ValidatorWarnings []string             `json:"validator_warnings,omitempty"`
}

type PluginDisplayDomain struct {
	Text       string   `json:"text,omitempty"`
	Unit       string   `json:"unit,omitempty"`
	Min        *float64 `json:"min,omitempty"`
	Max        *float64 `json:"max,omitempty"`
	Scale      string   `json:"scale,omitempty"`
	Status     string   `json:"status,omitempty"`
	Source     string   `json:"source,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
}

type PluginEvidence struct {
	Kind    string `json:"kind,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type PluginProvenance struct {
	Kind    string         `json:"kind,omitempty"`
	Source  string         `json:"source,omitempty"`
	Summary string         `json:"summary,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type PluginSkillOperation struct {
	Name        string         `json:"name"`
	Inputs      []string       `json:"inputs,omitempty"`
	Resolver    string         `json:"resolver,omitempty"`
	ComponentID string         `json:"component_id,omitempty"`
	Params      map[string]any `json:"params,omitempty"`
}

type PluginSkillPreferences struct {
	Explicit     map[string]any `json:"explicit,omitempty"`
	UsageSummary map[string]any `json:"usage_summary,omitempty"`
}

type PluginSkillLegacy struct {
	ProjectDefault map[string]any `json:"project_default,omitempty"`
	ProfilePatch   *ProfilePatch  `json:"profile_patch,omitempty"`
}

type TeachModeSession struct {
	SchemaVersion int                  `json:"schema_version"`
	SessionID     string               `json:"session_id,omitempty"`
	Mode          LearningMode         `json:"mode"`
	TrackID       string               `json:"track_id,omitempty"`
	PluginID      string               `json:"plugin_id,omitempty"`
	PluginName    string               `json:"plugin_name,omitempty"`
	Rows          []TeachModeRowRecord `json:"rows"`
}

type TeachModeRowState string

const (
	TeachModeRowReady     TeachModeRowState = "ready"
	TeachModeRowCapturing TeachModeRowState = "capturing"
	TeachModeRowCaptured  TeachModeRowState = "captured"
	TeachModeRowNoChange  TeachModeRowState = "no_change"
)

type TeachModeRowRecord struct {
	RowID             string                   `json:"row_id"`
	Description       string                   `json:"description,omitempty"`
	DisplayDomainText string                   `json:"display_domain_text,omitempty"`
	DisplayDomain     *PluginDisplayDomain     `json:"display_domain,omitempty"`
	State             TeachModeRowState        `json:"state"`
	BeforeSnapshot    *PluginParameterSnapshot `json:"before_snapshot,omitempty"`
	AfterSnapshot     *PluginParameterSnapshot `json:"after_snapshot,omitempty"`
	Changes           []PluginParameterChange  `json:"changes,omitempty"`
}

type PluginParameterSnapshot struct {
	CapturedAt string                         `json:"captured_at,omitempty"`
	TrackID    string                         `json:"track_id,omitempty"`
	PluginID   string                         `json:"plugin_id,omitempty"`
	PluginName string                         `json:"plugin_name,omitempty"`
	Parameters []PluginParameterSnapshotValue `json:"parameters"`
}

type PluginParameterSnapshotValue struct {
	ParamID         string  `json:"param_id"`
	Name            string  `json:"name,omitempty"`
	NormalizedValue float64 `json:"normalized_value"`
	ValueText       string  `json:"value_text,omitempty"`
}

type PluginParameterChange struct {
	ParamID          string  `json:"param_id"`
	Name             string  `json:"name,omitempty"`
	BeforeNormalized float64 `json:"before_normalized"`
	AfterNormalized  float64 `json:"after_normalized"`
	Delta            float64 `json:"delta"`
}
