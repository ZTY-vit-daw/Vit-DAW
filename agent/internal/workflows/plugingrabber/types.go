package plugingrabber

type LearningTarget struct {
	TrackID    string
	PluginID   string
	PluginName string
	Intent     string
}

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
	ParameterCount       int                    `json:"parameter_count"`
	QuickControls        []QuickControlDigest   `json:"quick_controls,omitempty"`
	RecommendedGroups    []RecommendedGroupInfo `json:"recommended_groups,omitempty"`
	Parameters           []ParameterInfo        `json:"parameters"`
}

type ParameterInfo struct {
	ID               string `json:"id"`
	Name             string `json:"name,omitempty"`
	RawName          string `json:"raw_param_name,omitempty"`
	Alias            string `json:"alias,omitempty"`
	DisplayGroup     string `json:"display_group,omitempty"`
	NormalizedRole   string `json:"normalized_role,omitempty"`
	ControlRelevance string `json:"control_relevance,omitempty"`
	HostControllable bool   `json:"host_controllable,omitempty"`
	IsBoolean        bool   `json:"is_boolean,omitempty"`
	IsDiscrete       bool   `json:"is_discrete,omitempty"`
	NumSteps         int    `json:"num_steps,omitempty"`
	Unit             string `json:"unit,omitempty"`
	Value            any    `json:"value,omitempty"`
	NormalizedValue  any    `json:"normalized_value,omitempty"`
	ValueText        string `json:"value_text,omitempty"`
	Min              any    `json:"min,omitempty"`
	Max              any    `json:"max,omitempty"`
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
}
