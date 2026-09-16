import { state, findCharacter } from './store.js';

export const $ = (id) => document.getElementById(id);

export function escapeHTML(value) {
  return String(value == null ? '' : value).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

// formatText renders a safe subset of markdown: **bold**, *italic*, `code`.
export function formatText(text) {
  let out = escapeHTML(text || '');
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  out = out.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>');
  out = out.replace(/\n/g, '<br>');
  return out;
}

export function openModal(templateId) {
  const template = $(templateId);
  const root = $('modal-root');
  if (!template || !root) return null;
  const node = template.content.firstElementChild.cloneNode(true);
  root.innerHTML = '';
  root.appendChild(node);
  const close = () => { root.innerHTML = ''; };
  node.querySelectorAll('[data-close]').forEach((btn) => btn.addEventListener('click', close));
  node.addEventListener('click', (event) => { if (event.target === node) close(); });
  return { node, close };
}

let toastWrap = null;
export function toast(message, type = 'info') {
  if (!toastWrap) {
    toastWrap = document.createElement('div');
    toastWrap.className = 'toast-wrap';
    document.body.appendChild(toastWrap);
  }
  const node = document.createElement('div');
  node.className = 'toast' + (type === 'error' ? ' error' : '');
  node.innerHTML = formatText(message);
  toastWrap.appendChild(node);
  setTimeout(() => {
    node.style.opacity = '0';
    node.style.transition = 'opacity .25s';
    setTimeout(() => node.remove(), 260);
  }, type === 'error' ? 6500 : 3800);
}

function avatarFor(message) {
  const session = state.session;
  if (message.role === 'user') {
    const persona = session && session.persona;
    if (persona && persona.name) return persona.name.trim().charAt(0).toUpperCase();
    return '🧍';
  }
  const character = findCharacter(message.speaker);
  if (character && character.avatar) return character.avatar;
  if (character && character.name) return character.name.trim().charAt(0).toUpperCase();
  switch (message.kind) {
    case 'narration': return '✦';
    case 'dice': return '🎲';
    case 'prompt': return '❯';
    case 'ooc': return '💬';
    default: return '◆';
  }
}

function messageNode(message) {
  const node = document.createElement('div');
  node.className = ['msg', message.role, message.kind, message.pending ? 'pending' : ''].filter(Boolean).join(' ');
  node.dataset.id = message.id;
  const speaker = message.speaker || (message.role === 'user' ? ((state.session && state.session.persona && state.session.persona.name) || 'You') : '');
  const showSpeaker = Boolean(speaker) && message.kind !== 'narration' && message.kind !== 'dice';
  node.innerHTML =
    '<div class="avatar">' + escapeHTML(avatarFor(message)) + '</div>' +
    '<div class="body">' +
      (showSpeaker || message.mood
        ? '<div class="speaker">' +
            (showSpeaker ? '<span>' + escapeHTML(speaker) + '</span>' : '') +
            (message.mood ? '<span class="mood">' + escapeHTML(message.mood) + '</span>' : '') +
          '</div>'
        : '') +
      '<div class="text">' + formatText(message.text) + '</div>' +
    '</div>';
  return node;
}

function nearBottom(el) {
  return el.scrollHeight - el.scrollTop - el.clientHeight < 140;
}

export function appendMessage(message) {
  const container = $('messages');
  if (!container) return;
  const stick = nearBottom(container);
  const empty = container.querySelector('.empty');
  if (empty) empty.remove();
  container.appendChild(messageNode(message));
  if (stick) container.scrollTop = container.scrollHeight;
}

export function renderMessages() {
  const container = $('messages');
  if (!container) return;
  container.innerHTML = '';
  const messages = (state.session && state.session.messages) || [];
  if (!messages.length) {
    renderEmptyState();
    return;
  }
  messages.forEach((m) => container.appendChild(messageNode(m)));
  container.scrollTop = container.scrollHeight;
}

export function renderEmptyState() {
  const container = $('messages');
  if (!container) return;
  container.innerHTML =
    '<div class="empty">' +
      '<h2>Begin a story</h2>' +
      '<p>Genesis is an agentic game master. The model never writes plain text: every beat, every ' +
      'scene change and every dice roll is a tool call, which is what makes it easy to extend.</p>' +
      '<p><button class="btn btn-primary" id="empty-new" type="button">Create your first story</button></p>' +
    '</div>';
  const btn = $('empty-new');
  if (btn) btn.addEventListener('click', () => document.dispatchEvent(new CustomEvent('genesis:new-story')));
}

export function renderSessions() {
  const list = $('session-list');
  if (!list) return;
  list.innerHTML = '';
  if (!state.sessions.length) {
    list.innerHTML = '<p class="muted" style="padding:10px">No stories yet.</p>';
    return;
  }
  const activeId = state.session && state.session.id;
  state.sessions.forEach((session) => {
    const item = document.createElement('button');
    item.type = 'button';
    item.className = 'session-item' + (session.id === activeId ? ' active' : '');
    item.innerHTML =
      '<span class="t">' + escapeHTML(session.title || 'Untitled') + '</span>' +
      '<span class="s">' + escapeHTML(session.character || '') + (session.messageCount ? ' · ' + session.messageCount + ' msg' : '') + '</span>';
    item.addEventListener('click', () => document.dispatchEvent(new CustomEvent('genesis:open-session', { detail: session.id })));
    list.appendChild(item);
  });
}

export function renderHeader() {
  const session = state.session;
  const title = $('chat-title');
  const subtitle = $('chat-subtitle');
  if (!title || !subtitle) return;
  title.textContent = (session && session.title) || 'Genesis';
  if (!session) {
    subtitle.textContent = 'agentic roleplay';
    return;
  }
  const bits = [];
  if (session.model) bits.push(session.model);
  if (session.characters && session.characters.length) bits.push(session.characters.map((c) => c.name).join(', '));
  if (state.usage && state.usage.totalTokens) bits.push(state.usage.totalTokens + ' tok');
  subtitle.textContent = bits.join(' · ') || 'agentic roleplay';
}

export function renderScene() {
  const bar = $('scene-bar');
  if (!bar) return;
  const scene = (state.session && state.session.scene) || {};
  const fields = [
    ['📍', scene.location],
    ['🕓', scene.time],
    ['☁', scene.weather],
  ].filter(([, v]) => v && String(v).trim());
  if (!fields.length && !(scene.notes || '').trim()) {
    bar.hidden = true;
    bar.innerHTML = '';
    return;
  }
  bar.hidden = false;
  bar.innerHTML = fields.map(([icon, v]) => '<span>' + icon + ' <b>' + escapeHTML(v) + '</b></span>').join('') +
    (scene.notes ? '<span class="muted">' + escapeHTML(scene.notes) + '</span>' : '');
}

export function renderStatus(status, step) {
  const bar = $('status-bar');
  if (!bar) return;
  if (!status) {
    bar.hidden = true;
    bar.innerHTML = '';
    return;
  }
  const label = status === 'thinking' ? 'Thinking' + (step ? ' · step ' + step : '') + '…' : escapeHTML(status);
  bar.hidden = false;
  bar.innerHTML = '<span class="dots"><i></i><i></i><i></i></span><span>' + label + '</span>';
}

export function renderChoices() {
  const box = $('choices');
  if (!box) return;
  if (!state.choices || !state.choices.length) {
    box.hidden = true;
    box.innerHTML = '';
    return;
  }
  box.hidden = false;
  box.innerHTML = '';
  state.choices.forEach((choice) => {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'choice';
    btn.innerHTML = escapeHTML(choice.text) + (choice.description ? '<small>' + escapeHTML(choice.description) + '</small>' : '');
    btn.addEventListener('click', () => document.dispatchEvent(new CustomEvent('genesis:choose', { detail: choice.text })));
    box.appendChild(btn);
  });
}

function activityNode(item) {
  const node = document.createElement('details');
  node.className = 'activity' + (item.error ? ' error' : '');
  const status = item.done ? (item.error ? '✕' : '✓') : '…';
  node.innerHTML =
    '<summary><span class="tool-name">' + escapeHTML(item.name) + '</span><span class="muted">' + status + '</span></summary>' +
    (item.args ? '<pre>' + escapeHTML(item.args) + '</pre>' : '') +
    (item.result ? '<pre>' + escapeHTML(item.result) + '</pre>' : '') +
    (item.error ? '<pre>' + escapeHTML(item.error) + '</pre>' : '');
  return node;
}

export function renderPanel() {
  const panelScene = $('panel-scene');
  const panelChars = $('panel-characters');
  const panelState = $('panel-state');
  const panelMemories = $('panel-memories');
  const panelActivity = $('panel-activity');
  const session = state.session;

  if (panelScene) {
    const scene = (session && session.scene) || {};
    const rows = ['location', 'time', 'weather', 'background', 'notes']
      .filter((k) => scene[k] && String(scene[k]).trim())
      .map((k) => '<div><b>' + k + ':</b> ' + escapeHTML(scene[k]) + '</div>');
    panelScene.innerHTML = rows.join('') || 'Not established yet.';
    panelScene.classList.toggle('muted', rows.length === 0);
  }

  if (panelChars) {
    const chars = (session && session.characters) || [];
    panelChars.innerHTML = chars.length ? '' : '<p class="muted">None yet.</p>';
    chars.forEach((c) => {
      const card = document.createElement('div');
      card.className = 'char-card';
      const stateChips = Object.entries(c.state || {}).map(([k, v]) => '<span class="chip">' + escapeHTML(k + ': ' + (typeof v === 'object' ? JSON.stringify(v) : v)) + '</span>');
      card.innerHTML =
        '<div class="avatar">' + escapeHTML(c.avatar || (c.name || '?').charAt(0).toUpperCase()) + '</div>' +
        '<div><div class="name">' + escapeHTML(c.name || 'Unnamed') + '</div>' +
        '<div class="desc">' + escapeHTML(c.description || c.personality || '') + '</div>' +
        (stateChips.length ? '<div class="chips">' + stateChips.join('') + '</div>' : '') +
        '</div>';
      panelChars.appendChild(card);
    });
  }

  if (panelState) {
    panelState.textContent = JSON.stringify((session && session.state) || {}, null, 2);
  }

  if (panelMemories) {
    const memories = ((session && session.memories) || []).slice().reverse();
    panelMemories.innerHTML = memories.length ? '' : '<p class="muted">Nothing remembered yet.</p>';
    memories.slice(0, 40).forEach((m) => {
      const row = document.createElement('div');
      row.className = 'mem-item';
      row.innerHTML = '<span class="imp">' + (m.importance || 3) + '</span>' + escapeHTML(m.content);
      panelMemories.appendChild(row);
    });
  }

  if (panelActivity) {
    panelActivity.innerHTML = state.activity.length ? '' : '<p class="muted">No tool calls yet.</p>';
    state.activity.slice(-20).reverse().forEach((item) => panelActivity.appendChild(activityNode(item)));
  }
}

export function toolsHTML(tools) {
  if (!tools || !tools.length) return '<p class="muted">No tools registered.</p>';
  return tools.map((t) =>
    '<div class="tool-item">' +
      '<div class="head"><span class="name">' + escapeHTML(t.name) + '</span>' +
      '<span class="cat">' + escapeHTML(t.category || 'tool') + '</span>' +
      (t.terminal ? '<span class="cat">terminal</span>' : '') +
      (t.query ? '<span class="cat">info</span>' : '') +
      '</div>' +
      '<p>' + escapeHTML(t.description || '') + '</p>' +
      '<details><summary class="muted">schema</summary><pre>' + escapeHTML(JSON.stringify(t.parameters || {}, null, 2)) + '</pre></details>' +
    '</div>'
  ).join('');
}

export function setStreaming(on) {
  const send = $('send');
  const stop = $('stop');
  if (send) send.hidden = on;
  if (stop) stop.hidden = !on;
  if (!on) renderStatus(null);
}
