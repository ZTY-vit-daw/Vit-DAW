// Command domreadiness projects evaluator-side source evidence through the
// current DOM implementation. It is a read-only analysis utility and has no
// Agent, project, plugin, PCA, or mutation authority.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"vit-daw-agent/internal/dom"
)

type request struct {
	Cases []caseInput `json:"cases"`
}

type caseInput struct {
	CaseID      string             `json:"case_id"`
	Project     string             `json:"project"`
	Target      string             `json:"target"`
	StartSecond float64            `json:"start_seconds"`
	EndSecond   float64            `json:"end_seconds"`
	Source      dom.SourceEvidence `json:"source"`
}

type caseOutput struct {
	CaseID      string         `json:"case_id"`
	Project     string         `json:"project"`
	Target      string         `json:"target"`
	StartSecond float64        `json:"start_seconds"`
	EndSecond   float64        `json:"end_seconds"`
	Projection  dom.Projection `json:"projection"`
}

func main() {
	var input request
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		fail(fmt.Errorf("decode request: %w", err))
	}
	if len(input.Cases) == 0 {
		fail(fmt.Errorf("at least one case is required"))
	}
	out := struct {
		SchemaVersion string       `json:"schema_version"`
		Cases         []caseOutput `json:"cases"`
	}{SchemaVersion: "semantic_processor_project_dom_readiness.v1"}
	for _, candidate := range input.Cases {
		projection := dom.Build(dom.Input{
			Mode:          dom.ModeSourceOnly,
			ObservationID: "offline_readiness_" + candidate.CaseID,
			TargetRef:     map[string]any{"kind": "track", "id": candidate.Target},
			Conditions: dom.Conditions{
				StartSeconds: candidate.StartSecond,
				EndSeconds:   candidate.EndSecond,
			},
			Source: candidate.Source,
		})
		out.Cases = append(out.Cases, caseOutput{
			CaseID: candidate.CaseID, Project: candidate.Project, Target: candidate.Target,
			StartSecond: candidate.StartSecond, EndSecond: candidate.EndSecond, Projection: projection,
		})
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fail(fmt.Errorf("encode response: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
