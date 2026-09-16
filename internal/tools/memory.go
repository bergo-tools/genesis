package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func rememberTool() *agent.Tool {
	return &agent.Tool{
		Name:        "remember",
		Category:    "memory",
		Description: "Store a durable fact the story should not forget: promises, secrets, injuries, relationships, discoveries.",
		Parameters: object(map[string]any{
			"content":    stringProp("The fact to remember, written as a standalone sentence."),
			"importance": intProp("1 (trivia) to 5 (crucial). Defaults to 3."),
			"tags":       arrayProp("Optional tags for later recall.", stringProp("tag")),
		}, "content"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Content    string   `json:"content"`
				Importance int      `json:"importance"`
				Tags       []string `json:"tags"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			content := strings.TrimSpace(a.Content)
			if content == "" {
				return nil, errors.New("content is required")
			}
			importance := a.Importance
			if importance < 1 || importance > 5 {
				importance = 3
			}
			m := &store.Memory{
				ID:         store.NewID(),
				Content:    content,
				Importance: importance,
				Tags:       a.Tags,
				CreatedAt:  time.Now().UTC(),
			}
			tc.Session.Memories = append(tc.Session.Memories, m)
			if tc.Emit != nil {
				tc.Emit(agent.Event{Type: agent.EventMemory, Step: tc.Step, Memory: m})
			}
			return map[string]any{"ok": true, "id": m.ID}, nil
		},
	}
}

func recallTool() *agent.Tool {
	return &agent.Tool{
		Name:        "recall",
		Category:    "info",
		Query:       true,
		Description: "Search long-term memory and, optionally, the recent transcript, for anything relevant to a query.",
		Parameters: object(map[string]any{
			"query": stringProp("What you are trying to remember."),
			"limit": intProp("Maximum results. Defaults to 5."),
		}, "query"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Query string `json:"query"`
				Limit int    `json:"limit"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			query := strings.TrimSpace(a.Query)
			if query == "" {
				return nil, errors.New("query is required")
			}
			limit := a.Limit
			if limit <= 0 {
				limit = 5
			}
			terms := tokenize(query)
			type hit struct {
				Memory *store.Memory `json:"memory"`
				Score  int           `json:"score"`
			}
			hits := make([]hit, 0, len(tc.Session.Memories))
			for _, m := range tc.Session.Memories {
				if m == nil {
					continue
				}
				score := overlap(terms, tokenize(m.Content)) + m.Importance
				if score > 0 {
					hits = append(hits, hit{Memory: m, Score: score})
				}
			}
			sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
			if len(hits) > limit {
				hits = hits[:limit]
			}
			return map[string]any{"query": query, "results": hits}, nil
		},
	}
}

func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r > 0x2e80)
	})
	out := fields[:0]
	for _, f := range fields {
		if len([]rune(f)) >= 2 {
			out = append(out, f)
		}
	}
	return out
}

func overlap(a, b []string) int {
	set := make(map[string]bool, len(b))
	for _, t := range b {
		set[t] = true
	}
	score := 0
	for _, t := range a {
		if set[t] {
			score++
		}
	}
	return score
}
