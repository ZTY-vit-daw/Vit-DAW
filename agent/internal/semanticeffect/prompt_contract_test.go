package semanticeffect

import (
	"encoding/json"
	"testing"
)

func TestStaticEQActionPromptExampleMatchesRuntimeValidation(t *testing.T) {
	var action Action
	if err := json.Unmarshal([]byte(StaticEQActionPromptExample), &action); err != nil {
		t.Fatalf("decode shared prompt example: %v", err)
	}
	if err := action.Validate(); err != nil {
		t.Fatalf("shared prompt example diverged from runtime validation: %v", err)
	}
}
