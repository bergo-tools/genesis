package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func sceneTool() *agent.Tool {
	return &agent.Tool{
		Name:     "scene",
		Category: "world",
		Description: "Set or update where the story is: location, in-fiction time, weather or atmosphere, " +
			"backdrop and persistent notes. Only the fields you provide change. Use it whenever the " +
			"scene moves somewhere new so the player can see the current setting.",
		Parameters: object(map[string]any{
			"location":   stringProp("Where the scene takes place."),
			"time":       stringProp("In-fiction time, e.g. dusk, three days later."),
			"weather":    stringProp("Weather or atmosphere."),
			"background": stringProp("A short visual description of the backdrop."),
			"notes":      stringProp("Any other persistent scene detail."),
			"last_call":  lastCallProp(),
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
			if summary := sceneSummary(*s); summary != "" {
				tc.Show(&store.Message{
					Role:    "assistant",
					Kind:    store.KindScene,
					Speaker: "Scene",
					Text:    summary,
					Args:    jsonBytes(*s),
				})
			}
			return map[string]any{"ok": true, "scene": *s}, nil
		},
	}
}

func sceneSummary(s store.Scene) string {
	parts := make([]string, 0, 3)
	for _, v := range []string{s.Location, s.Time, s.Weather} {
		if strings.TrimSpace(v) != "" {
			parts = append(parts, strings.TrimSpace(v))
		}
	}
	text := strings.Join(parts, " · ")
	if notes := strings.TrimSpace(s.Notes); notes != "" {
		if text != "" {
			text += " — "
		}
		text += notes
	}
	return text
}

func apply(dst *string, value string) {
	if v := strings.TrimSpace(value); v != "" {
		*dst = v
	}
}
