package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/zp/genesis/internal/agent"
)

func updateStateTool() *agent.Tool {
	return &agent.Tool{
		Name:     "update_state",
		Category: "state",
		Description: "Record durable information the player wants remembered: inventory, stats, flags, " +
			"relationships, promises, preferences, scene facts. path is a dot path such as " +
			"inventory.gold or cast.ilyra.trust. op is set, add, append, delete or toggle. This is " +
			"the only way to persist state, so use it whenever something should not be forgotten.",
		Parameters: object(map[string]any{
			"path": stringProp("Dot path into the story state, for example inventory.gold."),
			"op":   enumProp("Operation to perform.", "set", "add", "append", "delete", "toggle"),
			"value": map[string]any{
				"description": "Value for set/add/append. Ignored for delete and toggle.",
			},
			"last_call": lastCallProp(),
		}, "path", "op"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Path  string `json:"path"`
				Op    string `json:"op"`
				Value any    `json:"value"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			path := strings.TrimSpace(a.Path)
			if path == "" {
				return nil, errors.New("path is required")
			}
			if tc.Session.State == nil {
				tc.Session.State = map[string]any{}
			}
			op := strings.ToLower(strings.TrimSpace(a.Op))
			if op == "" {
				op = "set"
			}
			switch op {
			case "set":
				if err := setPath(tc.Session.State, path, a.Value); err != nil {
					return nil, err
				}
			case "add":
				cur, _ := getPath(tc.Session.State, path)
				base, _ := toFloat(cur)
				delta, ok := toFloat(a.Value)
				if !ok {
					return nil, errors.New("value must be numeric for add")
				}
				if err := setPath(tc.Session.State, path, numToAny(base+delta)); err != nil {
					return nil, err
				}
			case "append":
				cur, _ := getPath(tc.Session.State, path)
				list, _ := cur.([]any)
				list = append(list, a.Value)
				if err := setPath(tc.Session.State, path, list); err != nil {
					return nil, err
				}
			case "delete":
				deletePath(tc.Session.State, path)
			case "toggle":
				cur, _ := getPath(tc.Session.State, path)
				next := true
				if b, ok := cur.(bool); ok {
					next = !b
				}
				if err := setPath(tc.Session.State, path, next); err != nil {
					return nil, err
				}
			default:
				return nil, errors.New("unknown op: " + op)
			}
			if tc.Emit != nil {
				tc.Emit(agent.Event{Type: agent.EventState, Step: tc.Step, State: tc.Session.State})
			}
			v, _ := getPath(tc.Session.State, path)
			return map[string]any{"ok": true, "path": path, "value": v}, nil
		},
	}
}

func getPath(state map[string]any, path string) (any, bool) {
	if state == nil {
		return nil, false
	}
	if strings.TrimSpace(path) == "" {
		return state, true
	}
	var cur any = state
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func setPath(state map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return errors.New("path is required")
	}
	cur := state
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = value
			return nil
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	return nil
}

func deletePath(state map[string]any, path string) bool {
	parts := strings.Split(path, ".")
	cur := state
	for i, part := range parts {
		if i == len(parts)-1 {
			if _, ok := cur[part]; !ok {
				return false
			}
			delete(cur, part)
			return true
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			return false
		}
		cur = next
	}
	return false
}
