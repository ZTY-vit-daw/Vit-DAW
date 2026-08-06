package plugingrabber

type LoadTarget struct {
	TrackID     string
	PluginQuery string
	PluginPath  string
	Identifier  string
	PluginName  string
	Intent      string
	IntentKind  string
}

type LoadCandidate struct {
	Name         string
	Path         string
	Identifier   string
	Format       string
	Manufacturer string
	Category     string
	Score        int
	IsInstrument bool
}

type ParameterDigest struct {
	TrackID                   string                 `json:"track_id"`
	PluginID                  string                 `json:"plugin_id"`
	PluginName                string                 `json:"plugin_name,omitempty"`
	PluginIdentity            map[string]any         `json:"plugin_identity,omitempty"`
	TemplateRole              string                 `json:"template_role,omitempty"`
	PluginClass               string                 `json:"plugin_class,omitempty"`
	CurrentParamSignatureHash string                 `json:"current_param_signature_hash,omitempty"`
	ParameterCount            int                    `json:"parameter_count"`
	QuickControls             []QuickControlDigest   `json:"quick_controls,omitempty"`
	RecommendedGroups         []RecommendedGroupInfo `json:"recommended_groups,omitempty"`
	Parameters                []ParameterInfo        `json:"parameters"`
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
