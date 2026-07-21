package orchestration

import (
	"fmt"
	"sort"
	"strings"
)

// Registry is the deterministic capability admission boundary. It does not
// infer or execute a capability; callers must resolve an ID/version before a
// capability adapter is invoked.
type Registry struct {
	definitions map[string]CapabilityDefinition
}

func NewRegistry(definitions ...CapabilityDefinition) (*Registry, error) {
	r := &Registry{definitions: make(map[string]CapabilityDefinition)}
	for _, definition := range definitions {
		if err := r.Register(definition); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(definition CapabilityDefinition) error {
	if r == nil {
		return fmt.Errorf("nil capability registry")
	}
	definition.ID = strings.TrimSpace(definition.ID)
	definition.Version = strings.TrimSpace(definition.Version)
	definition.Family = strings.TrimSpace(definition.Family)
	if definition.ID == "" || definition.Version == "" || definition.Family == "" {
		return fmt.Errorf("capability id, version and family are required")
	}
	if r.definitions == nil {
		r.definitions = make(map[string]CapabilityDefinition)
	}
	key := registryKey(definition.ID, definition.Version)
	if _, exists := r.definitions[key]; exists {
		return fmt.Errorf("capability %s@%s already registered", definition.ID, definition.Version)
	}
	definition.Effects = uniqueSorted(definition.Effects)
	r.definitions[key] = definition
	return nil
}

func (r *Registry) Resolve(id, version string) (CapabilityDefinition, bool) {
	if r == nil {
		return CapabilityDefinition{}, false
	}
	definition, ok := r.definitions[registryKey(id, version)]
	return definition, ok
}

func (r *Registry) List() []CapabilityDefinition {
	if r == nil {
		return nil
	}
	out := make([]CapabilityDefinition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definition.Effects = append([]string(nil), definition.Effects...)
		out = append(out, definition)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID == out[j].ID {
			return out[i].Version < out[j].Version
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func registryKey(id, version string) string {
	return strings.TrimSpace(id) + "@" + strings.TrimSpace(version)
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			seen[value] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
