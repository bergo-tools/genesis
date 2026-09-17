// Headless smoke test: render the Preact tree and every modal to strings.
// Effects do not run, so this validates components and htm templates without a
// browser or any npm install.
import renderToString from './render-to-string.module.js';
import { h } from '../vendor/preact.module.js';
import { App } from '../app.js';
import { SettingsModal, MessageList, Composer, Choices } from '../components.js';
import { StoryModal } from '../story.js';
import { NewSessionModal, StoriesModal, PresetModal } from '../library.js';
import { Login } from '../login.js';
import { TokenStatus, TokenPanel } from '../tokens.js';
import { OptionPicker, modelOptions } from '../picker.js';

function check(name, out, needles) {
  const missing = needles.filter((needle) => !out.includes(needle));
  if (missing.length) {
    console.error(name + ' is missing: ' + missing.join(', '));
    process.exit(1);
  }
  return out.length;
}

function deny(name, out, needles) {
  const found = needles.filter((needle) => out.includes(needle));
  if (found.length) {
    console.error(name + ' should not contain: ' + found.join(', '));
    process.exit(1);
  }
  return 0;
}

const tools = [
  { name: 'message', description: 'say things', terminal: false, query: false },
  { name: 'think', description: 'inner thought', terminal: false, query: false },
  { name: 'choices', description: 'end turn', terminal: true, query: false },
];
const chatModels = [{ id: 'deepseek/deepseek-chat', name: 'DeepSeek', tools: true, context: 163840, maxOutput: 16000 }];
const stories = [
  { id: 'emberfall', title: 'Emberfall', description: 'dark fantasy', genre: 'fantasy', characterCount: 3, builtin: true },
  { id: 'custom1', title: 'Custom', description: 'mine', characterCount: 1, avatar: 'cover.png' },
];
const session = {
  id: 's1',
  storyTitle: 'Emberfall',
  settings: {},
  characters: [{ id: 'c1', name: 'Ilyra', description: 'archivist', voice: 'af_heart', avatar: '' }],
};

const chat = renderToString(h(MessageList, {
  session: {
    id: 's1',
    messages: [
      { id: 'u1', role: 'user', kind: 'user', text: 'hello there' },
      { id: 'n1', role: 'assistant', kind: 'narration', speaker: 'Narrator', text: 'The fog thickens.' },
      { id: 'a1', role: 'assistant', kind: 'speech', speaker: 'Ilyra', text: 'well met' },
      { id: 'a2', role: 'assistant', kind: 'action', speaker: 'Ilyra', text: 'She steps closer.' },
      { id: 'u2', role: 'user', kind: 'user', text: 'Draw the blade', choice: true },
      {
        id: 'a3', role: 'assistant', kind: 'speech', speaker: 'Ilyra', text: '她向前一步。站住。',
        blocks: [
          { type: 'text', text: '她向前一步。' },
          { type: 'thought', text: '他太年轻了。' },
          { type: 'text', text: '站住。' },
        ],
      },
    ],
  },
  onSpeak: () => {},
  onReroll: () => {},
  onEdit: () => {},
}));
const app = renderToString(h(App, { initialAuth: { enabled: false, authenticated: true } }));
const settings = renderToString(h(SettingsModal, { config: {}, tools, chatModels, disabledTools: [] }));
const sessionSettings = renderToString(h(StoryModal, { session }));
const newSession = renderToString(h(NewSessionModal, { stories }));
const storiesModal = renderToString(h(StoriesModal, { stories, onGenerate: () => {} }));
const preset = renderToString(h(PresetModal, { story: null, onGenerate: () => {} }));
const login = renderToString(h(Login, { onSuccess: () => {} }));
const composer = renderToString(h(Composer, { streaming: false, disabled: false, uploading: false, hasChoices: false, onSend: () => {}, onStop: () => {} }));
const composerMenu = renderToString(h(Composer, {
  streaming: false, disabled: false, uploading: false, hasChoices: false,
  onSend: () => {}, onStop: () => {}, menuInitiallyOpen: true,
}));
const tokenSession = {
  model: 'x/y',
  tokens: { lastPromptTokens: 5000, lastCachedTokens: 2000, totalPromptTokens: 12000, totalCompletionTokens: 800, requests: 3, totalCost: 0.0123 },
};
const tokenModels = [{ id: 'x/y', context: 20000, maxOutput: 4000 }];
const tokenStatus = renderToString(h(TokenStatus, { session: tokenSession, models: tokenModels, model: 'x/y' }));
const tokenPanel = renderToString(h(TokenPanel, { session: tokenSession, models: tokenModels, model: 'x/y' }));
const pickerOptions = modelOptions(chatModels);
const picker = renderToString(h(OptionPicker, {
  value: 'deepseek/deepseek-chat',
  options: pickerOptions,
  onChange: () => {},
  placeholder: 'Select a model',
}));
const choices = renderToString(h(Choices, {
  choices: [
    { text: 'Draw the blade' },
    { text: 'Slip away into the fog' },
  ],
  onChoose: () => {},
}));

// The narrator block must carry no avatar, and a repeated Ilyra beat must be
// grouped with the one before it.
const narratorBlock = chat.slice(chat.indexOf('data-id="n1"'), chat.indexOf('data-id="a1"'));
if (narratorBlock.includes('class="avatar"')) {
  console.error('Narrator must render without an avatar');
  process.exit(1);
}
const avatarCount = (chat.match(/class="avatar"/g) || []).length;
if (avatarCount !== 5) {
  console.error('expected 5 avatars (two user turns + three Ilyra beats), got ' + avatarCount);
  process.exit(1);
}

// A writing_block turn must keep the model's order: prose, thought, prose.
const blockOrder = ['她向前一步。', '他太年轻了。', '站住。'].map((s) => chat.indexOf(s));
if (!(blockOrder[0] >= 0 && blockOrder[0] < blockOrder[1] && blockOrder[1] < blockOrder[2])) {
  console.error('writing_block blocks rendered out of order: ' + blockOrder.join(', '));
  process.exit(1);
}

const total = check('MessageList', chat, ['hello there', 'well met', '✎', '↻', 'The fog thickens.', 'She steps closer.', 'msg assistant action grouped', 'choice-tag', 'msg user from-choice', 'Draw the blade', 'class="block"', 'class="thought-inline"', 'class="msg assistant"'])
  + check('App', app, ['Genesis', 'Begin a story', 'Create a story to start…', 'agentic roleplay', 'Generate a preset from a description'])
  + check('SettingsModal', settings, ['tool-toggles', 'toggle-row', 'picker-trigger', '<select'])
  + check('StoryModal', sessionSettings, ['Story settings', 'Story instructions', '+ Add character', 'picker-trigger', 'field-tag', 'marks what the model actually reads'])
  + deny('StoryModal', sessionSettings, ['tool-toggles', '<select', 'Temperature', 'Max output tokens'])
  + check('NewSessionModal', newSession, ['preset-card', 'New session', 'Emberfall', 'width:40px;height:40px', '<img', 'custom1/assets/cover.png'])
  + check('StoriesModal', storiesModal, ['preset-list', 'preset-row', 'New preset', 'Start', '✨ Generate'])
  + check('PresetModal', preset, ['New preset', 'Opening message', 'Story instructions', '+ Add character', 'preset-gen', 'Generate from a description', 'Nothing is saved until you press Save', 'field-tag', 'marks what the model actually reads'])
  + deny('PresetModal', preset, ['picker-trigger', 'tool-toggles', '<select', 'Temperature', 'Max output tokens', 'Default model'])
  + check('Login', login, ['login-screen', 'login-card', 'Password', 'Unlock'])
  + check('Composer', composer, ['What do you do?', 'composer-btn', 'aria-label="Send"', 'aria-label="More options"'])
  + deny('Composer', composer, ['OOC', 'composer-menu'])
  + check('ComposerMenu', composerMenu, ['composer-menu', 'pill-btn', 'OOC', '🖼 Image'])
  + check('TokenStatus', tokenStatus, ['token-status', 'token-bar', 'token-num', '5.0k/20k', '25% of 20k'])
  + check('TokenPanel', tokenPanel, ['stat-grid', 'Context', 'Cached', '5.0k / 20k (25%)', 'Session total', '13k', '$0.0123'])
  + check('OptionPicker', picker, ['picker-trigger', 'DeepSeek', 'deepseek/deepseek-chat'])
  + check('Choices', choices, ['choice-pager', 'choices-count', '1 / 2', 'choice-text', 'Draw the blade', 'choice-arrow', 'choice-dots']);
console.log('web SSR smoke test OK (' + total + ' chars across 14 renders)');
