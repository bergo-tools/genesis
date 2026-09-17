package store

import "time"

// BuiltinStories are the presets that ship with Genesis. They are created on
// first run and never overwritten afterwards.
func BuiltinStories() []*Story {
	return []*Story{emberfallStory()}
}

// EnsureBuiltins creates any builtin preset that does not exist yet.
func EnsureBuiltins(s *StoryStore) error {
	if s == nil {
		return nil
	}
	for _, b := range BuiltinStories() {
		if _, err := s.Get(b.ID); err == nil {
			continue
		} else if err != ErrNotFound {
			return err
		}
		if err := s.Create(b); err != nil {
			return err
		}
	}
	return nil
}

func emberfallStory() *Story {
	now := time.Now().UTC()
	return &Story{
		ID:          "emberfall",
		Title:       "灰烬王冠 · Emberfall",
		Description: "中世纪黑暗奇幻。王国的守焰结界正在熄灭，魔法从灰烬中苏醒；誓言、代价与被遗忘的真相是核心。",
		Genre:       "dark fantasy · magic",
		Persona: Persona{
			Description: "一位初出茅庐的咒术窃贼，全部家当是一本残缺的咒典。",
		},
		Opening: "灰烬落在你肩上，像迟到的雪。\n\n" +
			"通往灰烬堡的官道已经三天没有商队。路旁的界石上，守焰纹章正一寸寸剥落——" +
			"那是结界熄灭的征兆。\n\n" +
			"你怀里那本残缺的咒典忽然发烫。前方的雾里，有人正等着你。",
		Characters: []*Character{
			{
				ID:          "serelith",
				Name:        "瑟蕾丝·维恩",
				Description: "流亡的战场法师，靠替人解咒和卖假护符过活。",
				Personality: "冷静、刻薄、极度务实；从不解释自己的过去，只谈价钱。",
				CreatedAt:   now,
			},
			{
				ID:          "ordo",
				Name:        "奥尔多修士",
				Description: "灰烬之神的游方教士，背着一匣圣灰，声称能听见结界的心跳。",
				Personality: "温和、耐心、说话像布道；但每当问到灰烬从何而来，他就沉默。",
				CreatedAt:   now,
			},
			{
				ID:          "maerwyn",
				Name:        "梅尔温",
				Description: "破誓骑士，剑身上还刻着旧王的箴言。",
				Personality: "骄傲、寡言，把誓言当作枷锁；一旦立誓就绝不回头。",
				CreatedAt:   now,
			},
		},
		Settings: Settings{
			SystemPrompt: "这是一场黑暗中世纪奇幻冒险。魔法罕见、危险，且正在苏醒。" +
				"让施法有代价、让誓言与承诺被记住、让后果真实。用 narrator 铺陈环境与情节，" +
				"控制节奏：每回合推进一个场景节拍，然后给玩家选择。",
		},
		State: map[string]any{
			"kingdom": "灰烬堡",
			"ward":    "正在熄灭",
			"magic":   "正在苏醒",
			"place":   "雾中的官道，距灰烬堡三日路程",
			"time":    "黄昏",
			"player": map[string]any{
				"grimoire": "残缺的咒典",
				"coin":     7,
			},
		},
		Builtin:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
