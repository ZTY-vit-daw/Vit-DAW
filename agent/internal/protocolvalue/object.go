package protocolvalue

import "encoding/json"

// Object normalizes either an already generic JSON object or a typed Go
// struct/pointer crossing an internal protocol boundary. Harness results often
// retain strong types in-process even though the same HTTP payload appears as
// a JSON object to external clients.
func Object(value any) map[string]any {
	if value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil
	}
	return object
}
