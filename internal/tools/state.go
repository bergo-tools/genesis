package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func updateStateTool() *agent.Tool {
	return &agent.Tool{
		Name:     "update_state",
		Category: "state",
		Description: "Track durable facts about the story as key/value pairs: inventory, stats, flags, " +
			"promises, relationship values, clues, places. Pass every change from this turn in one call. " +
			"The tracked changes are shown to the player alongside your response and persist for the " +
			"whole session.",
		Parameters: object(map[string]any{
			"changes": arrayProp("Key/value changes to apply.", object(map[string]any{
				"key":   stringProp("Dot path, e.g. inventory.gold or cast.serelith.trust."),
				"value": map[string]any{"description": "The new value."},
				"op":    enumProp("Operation, defaults to set.", "set", "add", "append", "delete", "toggle"),
			}, "key")),
			"values": map[string]any{
				"type":                 "object",
				"description":          "Shorthand map of dot path to value (all set).",
				"additionalProperties": true,
			},
			"last_call": lastCallProp(),
		}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Changes []struct {
					Key   string `json:"key"`
					Value any    `json:"value"`
					Op    string `json:"op"`
				} `json:"changes"`
				Values map[string]any `json:"values"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if tc.Session.State == nil {
				tc.Session.State = map[string]any{}
			}
			tracked := map[string]any{}
			apply := func(key, op string, value any) error {
				key = strings.TrimSpace(key)
				if key == "" {
					return errors.New("key is required")
				}
				switch strings.ToLower(strings.TrimSpace(op)) {
				case "", "set":
					if err := setPath(tc.Session.State, key, value); err != nil {
						return err
					}
				case "add":
					cur, _ := getPath(tc.Session.State, key)
					base, _ := toFloat(cur)
					delta, ok := toFloat(value)
					if !ok {
						return errors.New("value must be numeric for add")
					}
					if err := setPath(tc.Session.State, key, numToAny(base+delta)); err != nil {
						return err
					}
				case "append":
					cur, _ := getPath(tc.Session.State, key)
					list, _ := cur.([]any)
					list = append(list, value)
					if err := setPath(tc.Session.State, key, list); err != nil {
						return err
					}
				case "delete":
					deletePath(tc.Session.State, key)
				case "toggle":
					cur, _ := getPath(tc.Session.State, key)
					next := true
					if b, ok := cur.(bool); ok {
						next = !b
					}
					if err := setPath(tc.Session.State, key, next); err != nil {
						return err
					}
				default:
					return errors.New("unknown op: " + op)
				}
				v, _ := getPath(tc.Session.State, key)
				tracked[key] = v
				return nil
			}
			for _, ch := range a.Changes {
				if err := apply(ch.Key, ch.Op, ch.Value); err != nil {
					return nil, err
				}
			}
			for k, v := range a.Values {
				if err := apply(k, "set", v); err != nil {
					return nil, err
				}
			}
			if len(tracked) == 0 {
				return nil, errors.New("no changes provided")
			}
			if tc.Emit != nil {
				tc.Emit(agent.Event{
					Type:    agent.EventState,
					Step:    tc.Step,
					State:   tc.Session.State,
					Tracker: tracked,
				})
			}
			// A compact, persistent record so the player sees what was tracked.
			tc.Show(&store.Message{
				Role:    "assistant",
				Kind:    store.KindState,
				Speaker: "Tracker",
				Text:    renderTracking(tracked),
				Args:    jsonBytes(tracked),
			})
			return map[string]any{"ok": true, "tracked": tracked}, nil
		},
	}
}

func renderTracking(tracked map[string]any) string {
	keys := make([]string, 0, len(tracked))
	for k := range tracked {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, k+" = "+string(jsonBytes(tracked[k])))
	}
	return strings.Join(lines, "\n")
}

func jsonBytes(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
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
