// Headless smoke test: render the Preact tree and the modals to strings.
// Effects do not run, so this validates components and htm templates without a
// browser or any npm install.
import renderToString from './render-to-string.module.js';
import { h } from '../vendor/preact.module.js';
import { App } from '../app.js';
import { SettingsModal, NewStoryModal } from '../components.js';
import { StoryModal } from '../story.js';

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
  { name: 'update_state', description: 'persist', terminal: false, query: false },
  { name: 'choices', description: 'end turn', terminal: true, query: false },
];
const chatModels = [{ id: 'deepseek/deepseek-chat', name: 'DeepSeek', tools: true, context: 163840, maxOutput: 16000 }];

const app = renderToString(h(App, {}));
const settings = renderToString(h(SettingsModal, { config: {}, tools, chatModels, disabledTools: [] }));
const newStory = renderToString(h(NewStoryModal, { config: {}, tools, chatModels, disabledTools: [] }));
const story = renderToString(h(StoryModal, { session: { id: 's1', characters: [], settings: {} }, tools, chatModels, disabledTools: [] }));

const total = check('App', app, ['Genesis', 'Begin a story', 'What do you do?', 'agentic roleplay'])
  + check('SettingsModal', settings, ['tool-toggles', 'toggle-row', 'chat-model-options', 'speech-model-options', '<select'])
  + check('NewStoryModal', newStory, ['tool-toggles', 'new-story-model-options', 'tool-toggles'])
  + check('StoryModal', story, ['tool-toggles', 'story-model-options', '<select']);
console.log('web SSR smoke test OK (' + total + ' chars across 4 renders)');
