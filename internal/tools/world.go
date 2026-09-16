package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func setSceneTool() *agent.Tool {
	return &agent.Tool{
		Name:        "set_scene",
		Category:    "world",
		Description: "Create or update the current scene. Only non-empty fields are changed.",
		Parameters: object(map[string]any{
			"location":   stringProp("Where the scene takes place."),
			"time":       stringProp("In-fiction time, e.g. dusk, three days later."),
			"weather":    stringProp("Weather or atmosphere."),
			"background": stringProp("A short visual description of the backdrop."),
			"notes":      stringProp("Any other persistent scene detail."),
		}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Location   string `json:"location"`
				Time       string `json:"time"`
				Weather    string `json:"weather"`
				Background string `json:"background"`
				Notes      string `json:"notes"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			s := &tc.Session.Scene
			apply(&s.Location, a.Location)
			apply(&s.Time, a.Time)
			apply(&s.Weather, a.Weather)
			apply(&s.Background, a.Background)
			apply(&s.Notes, a.Notes)
			if tc.Emit != nil {
				scene := *s
				tc.Emit(agent.Event{Type: agent.EventScene, Step: tc.Step, Scene: &scene})
			}
			return map[string]any{"ok": true, "scene": *s}, nil
		},
	}
}

func updateCharacterTool() *agent.Tool {
	return &agent.Tool{
		Name:     "update_character",
		Category: "world",
		Description: "Create a new character or update an existing one. Match by name (case-insensitive) " +
			"or id. Only non-empty fields are changed. Use the state field for mutable values such as " +
			"mood, affinity, or injuries.",
		Parameters: object(map[string]any{
			"name":        stringProp("Character name. Creates the character if it does not exist."),
			"id":          stringProp("Existing character id, if known."),
			"description": stringProp("Who the character is."),
			"personality": stringProp("Personality and mannerisms."),
			"appearance":  stringProp("Appearance."),
			"scenario":    stringProp("Their relationship to the story."),
			"mood":        stringProp("Shortcut for state.mood."),
			"avatar":      stringProp("Optional emoji or short label used as an avatar."),
			"add_tags":    arrayProp("Tags to add.", stringProp("tag")),
			"remove_tags": arrayProp("Tags to remove.", stringProp("tag")),
			"state":       map[string]any{"type": "object", "description": "State keys to merge in.", "additionalProperties": true},
		}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Name        string         `json:"name"`
				ID          string         `json:"id"`
				Description string         `json:"description"`
				Personality string         `json:"personality"`
				Appearance  string         `json:"appearance"`
				Scenario    string         `json:"scenario"`
				Mood        string         `json:"mood"`
				Avatar      string         `json:"avatar"`
				AddTags     []string       `json:"add_tags"`
				RemoveTags  []string       `json:"remove_tags"`
				State       map[string]any `json:"state"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			key := strings.TrimSpace(a.ID)
			if key == "" {
				key = strings.TrimSpace(a.Name)
			}
			c := tc.Session.FindCharacter(key)
			if c == nil {
				if strings.TrimSpace(a.Name) == "" {
					return nil, errors.New("name is required to create a character")
				}
				c = &store.Character{ID: store.NewID(), Name: strings.TrimSpace(a.Name), CreatedAt: time.Now().UTC()}
				tc.Session.Characters = append(tc.Session.Characters, c)
			}
			if n := strings.TrimSpace(a.Name); n != "" {
				c.Name = n
			}
			apply(&c.Description, a.Description)
			apply(&c.Personality, a.Personality)
			apply(&c.Appearance, a.Appearance)
			apply(&c.Scenario, a.Scenario)
			apply(&c.Avatar, a.Avatar)
			if len(a.AddTags) > 0 {
				c.Tags = mergeTags(c.Tags, a.AddTags, false)
			}
			if len(a.RemoveTags) > 0 {
				c.Tags = mergeTags(c.Tags, a.RemoveTags, true)
			}
			if c.State == nil {
				c.State = map[string]any{}
			}
			for k, v := range a.State {
				c.State[k] = v
			}
			if m := strings.TrimSpace(a.Mood); m != "" {
				c.State["mood"] = m
			}
			cc := *c
			if tc.Emit != nil {
				tc.Emit(agent.Event{Type: agent.EventCharacter, Step: tc.Step, Character: &cc})
			}
			return map[string]any{"ok": true, "character": cc}, nil
		},
	}
}

func updateStateTool() *agent.Tool {
	return &agent.Tool{
		Name:     "update_state",
		Category: "world",
		Description: "Change the persistent world state (inventory, flags, stats, clocks). path is a " +
			"dot path such as inventory.gold. op is set, add, append, delete, or toggle.",
		Parameters: object(map[string]any{
			"path":  stringProp("Dot path into the world state, e.g. inventory.gold."),
			"op":    enumProp("Operation to perform.", "set", "add", "append", "delete", "toggle"),
			"value": map[string]any{"description": "Value for set/add/append. Ignored for delete/toggle."},
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
			return map[string]any{"ok": true, "state": tc.Session.State}, nil
		},
	}
}

func getStateTool() *agent.Tool {
	return &agent.Tool{
		Name:        "get_state",
		Category:    "info",
		Query:       true,
		Description: "Read the world state, or one dot path within it.",
		Parameters:  object(map[string]any{"path": stringProp("Optional dot path. Omit to read everything.")}),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Path string `json:"path"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if strings.TrimSpace(a.Path) == "" {
				return map[string]any{"state": tc.Session.State}, nil
			}
			v, ok := getPath(tc.Session.State, strings.TrimSpace(a.Path))
			if !ok {
				return map[string]any{"found": false, "path": a.Path}, nil
			}
			return map[string]any{"found": true, "path": a.Path, "value": v}, nil
		},
	}
}

func apply(dst *string, value string) {
	if v := strings.TrimSpace(value); v != "" {
		*dst = v
	}
}

func mergeTags(existing, delta []string, remove bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(existing)+len(delta))
	for _, t := range existing {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	for _, t := range delta {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if remove {
			if !seen[key] {
				continue
			}
			seen[key] = false
			filtered := out[:0]
			for _, e := range out {
				if strings.ToLower(e) != key {
					filtered = append(filtered, e)
				}
			}
			out = filtered
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t)
	}
	return out
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
