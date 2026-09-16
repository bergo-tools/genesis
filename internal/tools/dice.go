package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/zp/genesis/internal/agent"
	"github.com/zp/genesis/internal/store"
)

func rollTool() *agent.Tool {
	return &agent.Tool{
		Name:     "roll",
		Category: "info",
		Query:    true,
		Description: "Roll dice for an uncertain outcome. notation is NdM with an optional modifier, " +
			"e.g. 2d6+3, 1d20-1. Use advantage or disadvantage to roll the whole set twice and take " +
			"the better or worse total.",
		Parameters: object(map[string]any{
			"notation":  stringProp("Dice expression such as 2d6+3 or 1d20."),
			"reason":    stringProp("What the roll is for."),
			"dc":        intProp("Optional difficulty class; the result reports success."),
			"advantage": enumProp("Roll mode.", "none", "advantage", "disadvantage"),
		}, "notation"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Notation  string `json:"notation"`
				Reason    string `json:"reason"`
				DC        int    `json:"dc"`
				Advantage string `json:"advantage"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			count, sides, mod, err := parseNotation(a.Notation)
			if err != nil {
				return nil, err
			}
			mode := strings.ToLower(strings.TrimSpace(a.Advantage))
			rollOnce := func() ([]int, int) {
				rolls := make([]int, count)
				total := 0
				for i := range rolls {
					rolls[i] = rand.IntN(sides) + 1
					total += rolls[i]
				}
				return rolls, total + mod
			}
			rolls, total := rollOnce()
			extra := []int(nil)
			if mode == "advantage" || mode == "disadvantage" {
				altRolls, altTotal := rollOnce()
				better := mode == "advantage" && altTotal > total
				worse := mode == "disadvantage" && altTotal < total
				if better || worse {
					extra = rolls
					rolls, total = altRolls, altTotal
				} else {
					extra = altRolls
				}
			}
			result := map[string]any{
				"notation": a.Notation,
				"rolls":    rolls,
				"modifier": mod,
				"total":    total,
				"reason":   a.Reason,
			}
			if extra != nil {
				result["alternate"] = extra
				result["mode"] = mode
			}
			if a.DC > 0 {
				result["dc"] = a.DC
				result["success"] = total >= a.DC
			}
			text := formatRoll(a.Notation, rolls, mod, total, a.DC, a.Reason)
			tc.Show(&store.Message{Role: "assistant", Kind: store.KindDice, Speaker: "Dice", Text: text, Args: mustRaw(result)})
			return result, nil
		},
	}
}

func parseNotation(notation string) (count, sides, modifier int, err error) {
	s := strings.ToLower(strings.TrimSpace(notation))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, 0, 0, errors.New("notation is required")
	}
	modifier = 0
	for _, sep := range []string{"+", "-"} {
		if idx := strings.LastIndex(s, sep); idx > 0 {
			n, convErr := strconv.Atoi(s[idx+1:])
			if convErr != nil {
				return 0, 0, 0, fmt.Errorf("invalid modifier in %q", notation)
			}
			if sep == "-" {
				n = -n
			}
			modifier = n
			s = s[:idx]
			break
		}
	}
	parts := strings.SplitN(s, "d", 2)
	count = 1
	if parts[0] != "" {
		count, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, 0, fmt.Errorf("invalid dice count in %q", notation)
		}
	}
	if len(parts) != 2 {
		return 0, 0, 0, fmt.Errorf("missing 'd' in %q", notation)
	}
	sides, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid die size in %q", notation)
	}
	if count < 1 || count > 100 {
		return 0, 0, 0, fmt.Errorf("dice count out of range in %q", notation)
	}
	if sides < 2 || sides > 1000 {
		return 0, 0, 0, fmt.Errorf("die size out of range in %q", notation)
	}
	return count, sides, modifier, nil
}

func formatRoll(notation string, rolls []int, modifier, total, dc int, reason string) string {
	parts := make([]string, len(rolls))
	for i, r := range rolls {
		parts[i] = strconv.Itoa(r)
	}
	expr := strings.Join(parts, ", ")
	if modifier != 0 {
		expr += fmt.Sprintf(" %+d", modifier)
	}
	text := fmt.Sprintf("🎲 %s → [%s] = %d", notation, expr, total)
	if dc > 0 {
		if total >= dc {
			text += fmt.Sprintf(" — success (DC %d)", dc)
		} else {
			text += fmt.Sprintf(" — failure (DC %d)", dc)
		}
	}
	if strings.TrimSpace(reason) != "" {
		text += " — " + reason
	}
	return text
}

func mustRaw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return b
}
