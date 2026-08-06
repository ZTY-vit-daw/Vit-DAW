package com

type ChangeDeltaInput struct {
	Before *Projection
	After  *Projection
}

type BehaviorChangeProjection struct {
	Status             string                    `json:"status"`
	BeforeProjectionID string                    `json:"before_projection_id"`
	AfterProjectionID  string                    `json:"after_projection_id"`
	Dimensions         []BehaviorDimensionChange `json:"dimensions"`
	EvidenceRefs       []string                  `json:"evidence_refs,omitempty"`
	Limitations        []string                  `json:"limitations,omitempty"`
}

type BehaviorDimensionChange struct {
	Dimension      string         `json:"dimension"`
	Status         string         `json:"status"`
	Classification string         `json:"classification"`
	Basis          string         `json:"basis,omitempty"`
	Confidence     float64        `json:"confidence"`
	BeforeStatus   string         `json:"before_status,omitempty"`
	AfterStatus    string         `json:"after_status,omitempty"`
	Metrics        map[string]any `json:"metrics,omitempty"`
	EvidenceRefs   []string       `json:"evidence_refs,omitempty"`
	Limitations    []string       `json:"limitations,omitempty"`
}
