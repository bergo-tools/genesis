// Package tools implements the builtin tool set. Registering a capability is a
// single agent.Registry.Register call, which keeps Genesis easy to extend.
package tools

import (
	"bytes"
	"encoding/json"
)

func object(props map[string]any, required ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		m["required"] = required
	}
	m["additionalProperties"] = false
	return m
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func enumProp(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func arrayProp(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}

// lastCallProp is added to every tool. The model sets it on the call it
// believes should end the turn; the agent honours it unless the choices tool
// is enabled, in which case choices must still close the turn.
func lastCallProp() map[string]any {
	return boolProp("Set true when you believe this is your final tool call for this turn. " +
		"It ends the turn, unless the choices tool is enabled, in which case you must still call choices.")
}

func decode(raw json.RawMessage, dst any) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	return dec.Decode(dst)
}
