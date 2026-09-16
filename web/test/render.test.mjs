// Headless smoke test: render the Preact tree and every modal to strings.
// Effects do not run, so this validates components and htm templates without a
// browser or any npm install.
import renderToString from './render-to-string.module.js';
import { h } from '../vendor/preact.module.js';
import { App } from '../app.js';
import { SettingsModal, MessageList } from '../components.js';
import { StoryModal } from '../story.js';
import { NewSessionModal, StoriesModal, PresetModal } from '../library.js';

function check(name, out, needles) {
  const missing = needles.filter((needle) => !out.includes(needle));
  if (missing.length) {
    console.error(name + ' is missing: ' + missing.join(', '));
    process.exit(1);
  }
  return out.length;
}

const tools = [
  { name: 'message', description: 'say things', terminal: false, query: false },
  { name: 'think', description: 'inner thought', terminal: false, query: false },
  { name: 'choices', description: 'end turn', terminal: true, query: false },
];
const chatModels = [{ id: 'deepseek/deepseek-chat', name: 'DeepSeek', tools: true, context: 163840, maxOutput: 16000 }];
const stories = [
  { id: 'emberfall', title: 'Emberfall', description: 'dark fantasy', genre: 'fantasy', characterCount: 3, builtin: true },
  { id: 'custom1', title: 'Custom', description: 'mine', characterCount: 1 },
];
const session = { id: 's1', storyTitle: 'Emberfall', characters: [], settings: {} };

const chat = renderToString(h(MessageList, {
  session: {
    id: 's1',
    messages: [
      { id: 'u1', role: 'user', kind: 'user', text: 'hello there' },
      { id: 'a1', role: 'assistant', kind: 'speech', speaker: 'Ilyra', text: 'well met' },
    ],
  },
  onSpeak: () => {},
  onReroll: () => {},
  onEdit: () => {},
}));
const app = renderToString(h(App, {}));
const settings = renderToString(h(SettingsModal, { config: {}, tools, chatModels, disabledTools: [] }));
const sessionSettings = renderToString(h(StoryModal, { session, tools, chatModels, disabledTools: [] }));
const newSession = renderToString(h(NewSessionModal, { stories }));
const storiesModal = renderToString(h(StoriesModal, { stories }));
const preset = renderToString(h(PresetModal, { story: null, config: {}, tools, chatModels }));

const total = check('MessageList', chat, ['hello there', 'well met', '✎', '↻'])
  + check('App', app, ['Genesis', 'Begin a story', 'What do you do?', 'agentic roleplay'])
  + check('SettingsModal', settings, ['tool-toggles', 'toggle-row', 'chat-model-options', 'speech-model-options', '<select'])
  + check('StoryModal', sessionSettings, ['tool-toggles', 'story-model-options', '<select'])
  + check('NewSessionModal', newSession, ['preset-card', 'New session', 'Emberfall'])
  + check('StoriesModal', storiesModal, ['preset-list', 'preset-row', 'New preset', 'Start'])
  + check('PresetModal', preset, ['tool-toggles', 'preset-model-options', '<select']);
console.log('web SSR smoke test OK (' + total + ' chars across 6 renders)');
