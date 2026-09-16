import { api } from './api.js';
import { state, activeCharacter, findCharacter } from './store.js';
import * as ui from './ui.js';

const LS_KEY = 'genesis.lastSession';

// ---------------------------------------------------------------- boot

async function boot() {
  wireStaticEvents();
  try {
    state.config = await api.config();
  } catch (err) {
    ui.toast('Could not load configuration: ' + err.message, 'error');
  }
  await refreshSessions();
  const last = localStorage.getItem(LS_KEY);
  const target = last && state.sessions.some((s) => s.id === last) ? last : (state.sessions[0] && state.sessions[0].id);
  if (target) {
    await openSession(target);
  } else {
    ui.renderEmptyState();
    ui.renderHeader();
    ui.renderPanel();
  }
  if (state.config && !state.config.hasKey) {
    ui.toast('Add your OpenRouter API key in Settings to begin.', 'info');
  }
}

function wireStaticEvents() {
  ui.$('new-session').addEventListener('click', openNewStory);
  ui.$('open-settings').addEventListener('click', openSettings);
  ui.$('open-tools').addEventListener('click', openTools);
  ui.$('send').addEventListener('click', () => send(ui.$('input').value));
  ui.$('stop').addEventListener('click', () => { if (state.abort) state.abort.abort(); });
  ui.$('menu-toggle').addEventListener('click', toggleSidebar);
  ui.$('panel-toggle').addEventListener('click', togglePanel);
  ui.$('panel-close').addEventListener('click', () => setPanel(false));
  ui.$('scrim').addEventListener('click', () => { setSidebar(false); setPanel(false); });

  const input = ui.$('input');
  input.addEventListener('input', autosize);
  input.addEventListener('keydown', (event) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      send(input.value);
    }
  });

  document.addEventListener('genesis:open-session', (e) => openSession(e.detail));
  document.addEventListener('genesis:new-story', openNewStory);
  document.addEventListener('genesis:choose', (e) => send(e.detail));
}

// ------------------------------------------------------------- layout

function setSidebar(open) {
  ui.$('sidebar').classList.toggle('open', open);
  ui.$('scrim').hidden = !(open || ui.$('panel').classList.contains('open'));
}
function toggleSidebar() { setSidebar(!ui.$('sidebar').classList.contains('open')); }
function setPanel(open) {
  ui.$('panel').classList.toggle('open', open);
  ui.$('scrim').hidden = !(open || ui.$('sidebar').classList.contains('open'));
}
function togglePanel() { setPanel(!ui.$('panel').classList.contains('open')); }
function closeDrawers() { setSidebar(false); setPanel(false); }

function autosize() {
  const input = ui.$('input');
  input.style.height = 'auto';
  input.style.height = Math.min(input.scrollHeight, Math.round(window.innerHeight * 0.4)) + 'px';
}

// ------------------------------------------------------------ sessions

async function refreshSessions() {
  try {
    const data = await api.sessions();
    state.sessions = data.sessions || [];
  } catch (err) {
    state.sessions = [];
  }
  ui.renderSessions();
}

async function openSession(id) {
  try {
    const session = await api.session(id);
    state.session = session;
    state.activity = [];
    state.choices = [];
    state.choicesPrompt = '';
    state.usage = null;
    localStorage.setItem(LS_KEY, id);
    ui.renderHeader();
    ui.renderScene();
    ui.renderMessages();
    ui.renderChoices();
    ui.renderPanel();
    ui.renderStatus(null);
    ui.renderSessions();
    closeDrawers();
  } catch (err) {
    ui.toast(err.message, 'error');
  }
}

async function deleteCurrent() {
  if (!state.session) return;
  if (!confirm('Delete this story? This cannot be undone.')) return;
  try {
    await api.deleteSession(state.session.id);
    state.session = null;
    localStorage.removeItem(LS_KEY);
    await refreshSessions();
    const next = state.sessions[0];
    if (next) await openSession(next.id);
    else {
      ui.renderMessages();
      ui.renderHeader();
      ui.renderScene();
      ui.renderPanel();
    }
  } catch (err) {
    ui.toast(err.message, 'error');
  }
}

function openSettings() {
  const modal = ui.openModal('tpl-settings');
  if (!modal) return;
  const cfg = state.config || {};
  ui.$('cfg-key').placeholder = cfg.hasKey ? '•••••••• (leave blank to keep)' : 'sk-or-...';
  ui.$('cfg-key-state').textContent = cfg.hasKey ? 'A key is configured.' : 'No key configured yet.';
  ui.$('cfg-base').value = cfg.baseUrl || '';
  ui.$('cfg-model').value = cfg.model || '';
  ui.$('cfg-temp').value = cfg.temperature != null ? cfg.temperature : 1;
  ui.$('cfg-maxtokens').value = cfg.maxTokens || 2048;
  ui.$('cfg-maxsteps').value = cfg.maxSteps || 8;
  ui.$('cfg-toolchoice').value = cfg.toolChoice || 'auto';
  ui.$('cfg-system').value = cfg.systemPrompt || '';

  ui.$('cfg-load-models').addEventListener('click', async () => {
    const btn = ui.$('cfg-load-models');
    btn.disabled = true;
    btn.textContent = '…';
    try {
      const data = await api.models();
      const list = ui.$('model-options');
      list.innerHTML = '';
      (data.models || []).slice(0, 400).forEach((m) => {
        const option = document.createElement('option');
        option.value = m.id;
        option.label = m.name;
        list.appendChild(option);
      });
      ui.toast((data.models || []).length + ' models loaded — start typing to filter.');
    } catch (err) {
      ui.toast(err.message, 'error');
    } finally {
      btn.disabled = false;
      btn.textContent = 'Load';
    }
  });

  ui.$('cfg-save').addEventListener('click', async () => {
    const body = {
      baseUrl: ui.$('cfg-base').value.trim(),
      model: ui.$('cfg-model').value.trim(),
      temperature: Number(ui.$('cfg-temp').value),
      maxTokens: Number(ui.$('cfg-maxtokens').value),
      maxSteps: Number(ui.$('cfg-maxsteps').value),
      toolChoice: ui.$('cfg-toolchoice').value,
      systemPrompt: ui.$('cfg-system').value,
    };
    const key = ui.$('cfg-key').value.trim();
    if (key) body.apiKey = key;
    try {
      state.config = await api.saveConfig(body);
      ui.toast('Settings saved.');
      modal.close();
    } catch (err) {
      ui.toast(err.message, 'error');
    }
  });
}

function openNewStory() {
  const modal = ui.openModal('tpl-new');
  if (!modal) return;
  ui.$('ns-create').addEventListener('click', async () => {
    const body = {
      title: ui.$('ns-title').value.trim(),
      persona: {
        name: ui.$('ns-persona-name').value.trim(),
        description: ui.$('ns-persona-desc').value.trim(),
      },
      character: {
        name: ui.$('ns-char-name').value.trim(),
        avatar: ui.$('ns-char-avatar').value.trim(),
        description: ui.$('ns-char-desc').value.trim(),
        personality: ui.$('ns-char-personality').value.trim(),
        scenario: ui.$('ns-char-scenario').value.trim(),
        greeting: ui.$('ns-char-greeting').value.trim(),
      },
    };
    try {
      const session = await api.createSession(body);
      modal.close();
      state.session = session;
      state.activity = [];
      state.choices = [];
      localStorage.setItem(LS_KEY, session.id);
      ui.renderHeader();
      ui.renderScene();
      ui.renderMessages();
      ui.renderPanel();
      ui.renderChoices();
      await refreshSessions();
      closeDrawers();
      if (!session.messages || !session.messages.length) {
        await runStream('/api/sessions/' + session.id + '/opening', {});
      }
    } catch (err) {
      ui.toast(err.message, 'error');
    }
  });
}

async function openTools() {
  const modal = ui.openModal('tpl-tools');
  if (!modal) return;
  const list = ui.$('tools-list');
  try {
    const data = await api.tools();
    state.tools = data.tools || [];
    list.innerHTML = ui.toolsHTML(state.tools);
  } catch (err) {
    list.innerHTML = '<p class="muted">' + ui.escapeHTML(err.message) + '</p>';
  }
}

// ------------------------------------------------------------ streaming

async function send(text) {
  text = (text || '').trim();
  if (!text || state.streaming || !state.session) return;
  state.choices = [];
  state.choicesPrompt = '';
  ui.renderChoices();
  const pending = {
    id: 'pending-' + Date.now(),
    role: 'user',
    kind: 'user',
    speaker: (state.session.persona && state.session.persona.name) || '',
    text,
    pending: true,
    createdAt: new Date().toISOString(),
  };
  state.session.messages.push(pending);
  ui.appendMessage(pending);
  ui.$('input').value = '';
  autosize();
  await runStream('/api/sessions/' + state.session.id + '/messages', { text });
}

async function runStream(path, body) {
  if (state.streaming) return;
  const controller = new AbortController();
  state.abort = controller;
  state.streaming = true;
  ui.setStreaming(true);
  try {
    await api.stream(path, body, handleEvent, controller.signal);
  } catch (err) {
    if (err.name !== 'AbortError') ui.toast(err.message, 'error');
  } finally {
    state.streaming = false;
    state.abort = null;
    ui.setStreaming(false);
    await refreshSessions();
  }
}

function reconcileUser(message) {
  if (!state.session) return;
  const index = state.session.messages.findIndex((m) => m.pending);
  if (index >= 0) state.session.messages[index] = message;
  else state.session.messages.push(message);
  ui.renderMessages();
}

function upsertCharacter(character) {
  if (!state.session || !character) return;
  state.session.characters = state.session.characters || [];
  const index = state.session.characters.findIndex((c) => c.id === character.id || c.name === character.name);
  if (index >= 0) state.session.characters[index] = character;
  else state.session.characters.push(character);
}

function addActivity(item) {
  state.activity.push(item);
  if (state.activity.length > 60) state.activity.shift();
  ui.renderPanel();
}

function finishActivity(tool) {
  const item = state.activity.find((a) => a.id === tool.id && !a.done) ||
    state.activity.find((a) => a.name === tool.name && !a.done);
  if (item) {
    item.done = true;
    item.error = tool.error || '';
    item.result = tool.result ? JSON.stringify(tool.result, null, 2) : '';
  } else {
    state.activity.push({ id: tool.id || tool.name, name: tool.name, done: true, error: tool.error || '', result: tool.result ? JSON.stringify(tool.result, null, 2) : '' });
  }
  ui.renderPanel();
}

function handleEvent(event) {
  switch (event.type) {
    case 'status':
      ui.renderStatus(event.status, event.step);
      break;
    case 'user_message':
      reconcileUser(event.message);
      break;
    case 'title':
      if (state.session) {
        state.session.title = event.title;
        ui.renderHeader();
      }
      break;
    case 'message':
      if (event.message && state.session) {
        state.session.messages.push(event.message);
        ui.appendMessage(event.message);
      }
      break;
    case 'scene':
      if (state.session) {
        state.session.scene = event.scene;
        ui.renderScene();
        ui.renderPanel();
      }
      break;
    case 'character':
      upsertCharacter(event.character);
      ui.renderPanel();
      break;
    case 'memory':
      if (state.session && event.memory) {
        state.session.memories = state.session.memories || [];
        state.session.memories.push(event.memory);
        ui.renderPanel();
      }
      break;
    case 'state':
      if (state.session) {
        state.session.state = event.state;
        ui.renderPanel();
      }
      break;
    case 'choices':
      state.choices = event.choices || [];
      state.choicesPrompt = event.prompt || '';
      ui.renderChoices();
      break;
    case 'reasoning':
      addActivity({ id: 'think-' + Date.now(), name: 'think', done: true, result: event.text });
      break;
    case 'tool_start':
      addActivity({ id: event.tool.id || (event.tool.name + '-' + Date.now()), name: event.tool.name, args: event.tool.args ? JSON.stringify(event.tool.args, null, 2) : '', done: false });
      break;
    case 'tool_end':
      finishActivity(event.tool);
      break;
    case 'usage':
      state.usage = event.usage;
      ui.renderHeader();
      break;
    case 'notice':
      ui.toast(event.text);
      break;
    case 'error':
      ui.toast(event.error, 'error');
      break;
    case 'turn_end':
      ui.renderStatus(null);
      break;
    default:
      console.debug('genesis: unhandled event', event);
  }
}

// Expose delete for keyboard shortcut.
window.addEventListener('keydown', (event) => {
  if (event.key === 'Delete' && (event.metaKey || event.ctrlKey)) deleteCurrent();
});

boot();
