import { h } from './vendor/preact.module.js';
import { useEffect, useRef, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { formatText, initial } from './format.js';

const html = htm.bind(h);

function avatarText(message, session) {
  if (message.role === 'user') {
    const name = session && session.persona && session.persona.name;
    return name ? initial(name) : '🧍';
  }
  const chars = (session && session.characters) || [];
  const speaker = String(message.speaker || '').toLowerCase();
  const found = chars.find((c) => String(c.name || '').toLowerCase() === speaker);
  if (found && found.avatar) return found.avatar;
  if (found && found.name) return initial(found.name);
  switch (message.kind) {
    case 'narration': return '✦';
    case 'dice': return '🎲';
    case 'prompt': return '❯';
    case 'ooc': return '💬';
    default: return '◆';
  }
}

export function Message({ message, session }) {
  const cls = ['msg', message.role, message.kind, message.pending ? 'pending' : ''].filter(Boolean).join(' ');
  const speaker = message.speaker || (message.role === 'user'
    ? ((session && session.persona && session.persona.name) || 'You')
    : '');
  const showSpeaker = speaker && message.kind !== 'narration' && message.kind !== 'dice';
  return html`
    <div class=${cls} data-id=${message.id}>
      <div class="avatar">${avatarText(message, session)}</div>
      <div class="body">
        ${(showSpeaker || message.mood) && html`
          <div class="speaker">
            ${showSpeaker && html`<span>${speaker}</span>`}
            ${message.mood && html`<span class="mood">${message.mood}</span>`}
          </div>`}
        <div class="text" dangerouslySetInnerHTML=${{ __html: formatText(message.text) }}></div>
      </div>
    </div>`;
}

export function MessageList({ session, streaming, onNewStory }) {
  const ref = useRef(null);
  const messages = (session && session.messages) || [];
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const stick = el.scrollHeight - el.scrollTop - el.clientHeight < 220;
    if (stick) el.scrollTop = el.scrollHeight;
  }, [messages.length, streaming]);
  if (!messages.length) {
    return html`
      <section class="messages" ref=${ref}>
        <div class="empty">
          <h2>Begin a story</h2>
          <p>Genesis is an agentic game master. The model never writes plain text: every beat, scene change and dice roll is a tool call, which is what makes it easy to extend.</p>
          <p><button class="btn btn-primary" type="button" onClick=${onNewStory}>Create your first story</button></p>
        </div>
      </section>`;
  }
  return html`
    <section class="messages" ref=${ref}>
      ${messages.map((m) => html`<${Message} key=${m.id} message=${m} session=${session} />`)}
    </section>`;
}

export function SceneBar({ scene }) {
  const s = scene || {};
  const fields = [['\u{1F4CD}', s.location], ['\u{1F553}', s.time], ['\u2601', s.weather]]
    .filter(([, v]) => v && String(v).trim());
  if (!fields.length && !(s.notes || '').trim()) return null;
  return html`
    <section class="scene-bar">
      ${fields.map(([icon, value]) => html`<span key=${icon}>${icon} <b>${value}</b></span>`)}
      ${s.notes && html`<span class="muted">${s.notes}</span>`}
    </section>`;
}

export function StatusBar({ status, step }) {
  if (!status) return null;
  const label = status === 'thinking' ? 'Thinking' + (step ? ' · step ' + step : '') + '…' : status;
  return html`
    <section class="status-bar">
      <span class="dots"><i></i><i></i><i></i></span><span>${label}</span>
    </section>`;
}

export function Choices({ choices, onChoose }) {
  if (!choices || !choices.length) return null;
  return html`
    <section class="choices">
      ${choices.map((choice, index) => html`
        <button key=${index} type="button" class="choice" onClick=${() => onChoose(choice.text)}>
          ${choice.text}
          ${choice.description && html`<small>${choice.description}</small>`}
        </button>`)}
    </section>`;
}

export function Composer({ streaming, disabled, onSend, onStop }) {
  const ref = useRef(null);
  const [value, setValue] = useState('');
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, Math.round(window.innerHeight * 0.4)) + 'px';
  }, [value]);
  const submit = () => {
    const text = value.trim();
    if (!text || streaming) return;
    setValue('');
    onSend(text);
  };
  return html`
    <footer class="composer">
      <textarea
        ref=${ref}
        rows="1"
        placeholder="What do you do?"
        autocomplete="off"
        value=${value}
        onInput=${(e) => setValue(e.currentTarget.value)}
        onKeyDown=${(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } }}
      ></textarea>
      ${streaming
        ? html`<button class="btn btn-danger" type="button" onClick=${onStop}>Stop</button>`
        : html`<button class="btn btn-primary" type="button" disabled=${Boolean(disabled)} onClick=${submit}>Send</button>`}
    </footer>`;
}

export function Sidebar({ sessions, activeId, open, onOpen, onNew, onSettings, onTools }) {
  return html`
    <aside class=${'sidebar' + (open ? ' open' : '')} aria-label="Sessions">
      <div class="sidebar-head">
        <span class="brand"><span class="brand-mark">◆</span> Genesis</span>
        <button class="btn btn-primary btn-sm" type="button" onClick=${onNew}>New story</button>
      </div>
      <nav class="session-list">
        ${!sessions.length && html`<p class="muted" style="padding:10px">No stories yet.</p>`}
        ${sessions.map((s) => html`
          <button
            key=${s.id}
            type="button"
            class=${'session-item' + (s.id === activeId ? ' active' : '')}
            onClick=${() => onOpen(s.id)}
          >
            <span class="t">${s.title || 'Untitled'}</span>
            <span class="s">${(s.character || '') + (s.messageCount ? ' · ' + s.messageCount + ' msg' : '')}</span>
          </button>`)}
      </nav>
      <div class="sidebar-foot">
        <button class="btn btn-ghost btn-sm" type="button" onClick=${onTools}>Tools</button>
        <button class="btn btn-ghost btn-sm" type="button" onClick=${onSettings}>Settings</button>
      </div>
    </aside>`;
}

export function Topbar({ session, usage, onMenu, onPanel }) {
  const bits = [];
  if (session && session.model) bits.push(session.model);
  if (session && session.characters && session.characters.length) bits.push(session.characters.map((c) => c.name).join(', '));
  if (usage && usage.totalTokens) bits.push(usage.totalTokens + ' tok');
  return html`
    <header class="topbar">
      <button id="menu-toggle" class="icon-btn" type="button" aria-label="Toggle sessions" onClick=${onMenu}>\u2630</button>
      <div class="topbar-title">
        <span id="chat-title">${(session && session.title) || 'Genesis'}</span>
        <small id="chat-subtitle">${bits.join(' · ') || 'agentic roleplay'}</small>
      </div>
      <button class="icon-btn" type="button" aria-label="Toggle world panel" onClick=${onPanel}>\u25E7</button>
    </header>`;
}

export function Panel({ session, activity, open, onClose }) {
  const scene = (session && session.scene) || {};
  const sceneRows = ['location', 'time', 'weather', 'background', 'notes']
    .filter((k) => scene[k] && String(scene[k]).trim())
    .map((k) => html`<div key=${k}><b>${k}:</b> ${scene[k]}</div>`);
  const chars = (session && session.characters) || [];
  const memories = ((session && session.memories) || []).slice().reverse().slice(0, 40);
  return html`
    <aside class=${'panel' + (open ? ' open' : '')} aria-label="World state">
      <div class="panel-head">
        <span>World</span>
        <button class="icon-btn" type="button" aria-label="Close" onClick=${onClose}>×</button>
      </div>
      <div class="panel-body">
        <section class="panel-section">
          <h3>Scene</h3>
          <div class=${'panel-scene' + (sceneRows.length ? '' : ' muted')}>${sceneRows.length ? sceneRows : 'Not established yet.'}</div>
        </section>
        <section class="panel-section">
          <h3>Characters</h3>
          ${!chars.length
            ? html`<p class="muted">None yet.</p>`
            : chars.map((c) => html`
              <div class="char-card" key=${c.id || c.name}>
                <div class="avatar">${c.avatar || initial(c.name)}</div>
                <div>
                  <div class="name">${c.name || 'Unnamed'}</div>
                  <div class="desc">${c.description || c.personality || ''}</div>
                  ${c.state && Object.keys(c.state).length > 0 && html`
                    <div class="chips">
                      ${Object.entries(c.state).map(([k, v]) => html`
                        <span class="chip" key=${k}>${k + ': ' + (typeof v === 'object' ? JSON.stringify(v) : v)}</span>`)}
                    </div>`}
                </div>
              </div>`)}
        </section>
        <section class="panel-section">
          <h3>World state</h3>
          <pre class="state-json">${JSON.stringify((session && session.state) || {}, null, 2)}</pre>
        </section>
        <section class="panel-section">
          <h3>Memories</h3>
          ${!memories.length
            ? html`<p class="muted">Nothing remembered yet.</p>`
            : memories.map((m) => html`<div class="mem-item" key=${m.id}><span class="imp">${m.importance || 3}</span>${m.content}</div>`)}
        </section>
        <section class="panel-section">
          <h3>Agent activity</h3>
          <div class="activity-list">
            ${!activity.length
              ? html`<p class="muted">No tool calls yet.</p>`
              : activity.slice(-20).reverse().map((item) => html`
                <details class=${'activity' + (item.error ? ' error' : '')} key=${item.id}>
                  <summary>
                    <span class="tool-name">${item.name}</span>
                    <span class="muted">${item.done ? (item.error ? '✕' : '✓') : '…'}</span>
                  </summary>
                  ${item.args && html`<pre>${item.args}</pre>`}
                  ${item.result && html`<pre>${item.result}</pre>`}
                  ${item.error && html`<pre>${item.error}</pre>`}
                </details>`)}
          </div>
        </section>
      </div>
    </aside>`;
}

export function SettingsModal({ config, onClose, onSave, onLoadModels }) {
  const cfg = config || {};
  const [form, setForm] = useState({
    apiKey: '',
    baseUrl: cfg.baseUrl || '',
    model: cfg.model || '',
    temperature: cfg.temperature != null ? cfg.temperature : 1,
    maxTokens: cfg.maxTokens || 2048,
    maxSteps: cfg.maxSteps || 8,
    toolChoice: cfg.toolChoice || 'auto',
    systemPrompt: cfg.systemPrompt || '',
  });
  const [models, setModels] = useState([]);
  const [busy, setBusy] = useState(false);
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.currentTarget.value }));
  const load = async () => {
    setBusy(true);
    try {
      setModels(await onLoadModels());
    } finally {
      setBusy(false);
    }
  };
  const save = () => {
    const body = {
      baseUrl: form.baseUrl.trim(),
      model: form.model.trim(),
      temperature: Number(form.temperature),
      maxTokens: Number(form.maxTokens),
      maxSteps: Number(form.maxSteps),
      toolChoice: form.toolChoice,
      systemPrompt: form.systemPrompt,
    };
    if (form.apiKey.trim()) body.apiKey = form.apiKey.trim();
    onSave(body);
  };
  return html`
    <div class="modal" onClick=${(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div class="modal-card">
        <header class="modal-head">
          <h2>Settings</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button>
        </header>
        <div class="modal-body">
          <label class="field"><span>API key</span>
            <input type="password" placeholder=${cfg.hasKey ? '•••••••• (leave blank to keep)' : 'sk-or-...'} value=${form.apiKey} onInput=${set('apiKey')} />
          </label>
          <p class="hint">${cfg.hasKey ? 'A key is configured. Leave blank to keep it.' : 'No key configured yet.'}</p>
          <label class="field"><span>Base URL</span><input type="text" value=${form.baseUrl} onInput=${set('baseUrl')} /></label>
          <div class="row">
            <label class="field grow"><span>Model</span><input type="text" list="model-options" value=${form.model} onInput=${set('model')} /></label>
            <button class="btn btn-ghost btn-sm" type="button" disabled=${busy} onClick=${load}>${busy ? '…' : 'Load'}</button>
          </div>
          <datalist id="model-options">
            ${models.map((m) => html`<option value=${m.id} label=${m.name} key=${m.id}></option>`)}
          </datalist>
          <div class="row">
            <label class="field"><span>Temperature</span><input type="number" min="0" max="2" step="0.05" value=${form.temperature} onInput=${set('temperature')} /></label>
            <label class="field"><span>Max tokens</span><input type="number" min="64" step="64" value=${form.maxTokens} onInput=${set('maxTokens')} /></label>
            <label class="field"><span>Max steps</span><input type="number" min="1" max="40" value=${form.maxSteps} onInput=${set('maxSteps')} /></label>
          </div>
          <label class="field"><span>Tool choice</span>
            <select value=${form.toolChoice} onChange=${set('toolChoice')}>
              <option value="auto">auto</option>
              <option value="required">required (force a tool call)</option>
              <option value="none">none</option>
            </select>
          </label>
          <label class="field"><span>Global director instructions</span>
            <textarea rows="4" placeholder="Extra standing instructions appended to every system prompt." value=${form.systemPrompt} onInput=${set('systemPrompt')}></textarea>
          </label>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" onClick=${save}>Save</button>
        </footer>
      </div>
    </div>`;
}

export function NewStoryModal({ onClose, onCreate }) {
  const [form, setForm] = useState({
    title: '', personaName: '', personaDesc: '',
    name: '', avatar: '', description: '', personality: '', scenario: '', greeting: '',
  });
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.currentTarget.value }));
  const create = () => {
    onCreate({
      title: form.title.trim(),
      persona: { name: form.personaName.trim(), description: form.personaDesc.trim() },
      character: {
        name: form.name.trim(),
        avatar: form.avatar.trim(),
        description: form.description.trim(),
        personality: form.personality.trim(),
        scenario: form.scenario.trim(),
        greeting: form.greeting.trim(),
      },
    });
  };
  return html`
    <div class="modal" onClick=${(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div class="modal-card">
        <header class="modal-head">
          <h2>New story</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button>
        </header>
        <div class="modal-body">
          <label class="field"><span>Title</span><input type="text" placeholder="Optional" value=${form.title} onInput=${set('title')} /></label>
          <div class="grid-2">
            <label class="field"><span>Your name</span><input type="text" placeholder="The player" value=${form.personaName} onInput=${set('personaName')} /></label>
            <label class="field"><span>Your description</span><input type="text" placeholder="A wandering cartographer" value=${form.personaDesc} onInput=${set('personaDesc')} /></label>
          </div>
          <hr>
          <div class="grid-2">
            <label class="field"><span>Character name</span><input type="text" placeholder="Ilyra" value=${form.name} onInput=${set('name')} /></label>
            <label class="field"><span>Avatar (emoji)</span><input type="text" maxlength="4" placeholder="🜁" value=${form.avatar} onInput=${set('avatar')} /></label>
          </div>
          <label class="field"><span>Description</span><textarea rows="2" placeholder="An archivist who guards a drowned library." value=${form.description} onInput=${set('description')}></textarea></label>
          <label class="field"><span>Personality</span><textarea rows="2" placeholder="Dry, patient, secretly sentimental." value=${form.personality} onInput=${set('personality')}></textarea></label>
          <label class="field"><span>Scenario</span><textarea rows="2" placeholder="The player washed ashore during a storm." value=${form.scenario} onInput=${set('scenario')}></textarea></label>
          <label class="field"><span>Opening message</span><textarea rows="2" placeholder="Leave blank to let the agent open the scene." value=${form.greeting} onInput=${set('greeting')}></textarea></label>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" onClick=${create}>Create</button>
        </footer>
      </div>
    </div>`;
}

export function ToolsModal({ tools, onClose }) {
  return html`
    <div class="modal" onClick=${(e) => { if (e.target === e.currentTarget) onClose(); }}>
      <div class="modal-card">
        <header class="modal-head">
          <h2>Registered tools</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button>
        </header>
        <div class="modal-body">
          <div class="tools-list">
            ${!tools.length
              ? html`<p class="muted">No tools registered.</p>`
              : tools.map((t) => html`
                <div class="tool-item" key=${t.name}>
                  <div class="head">
                    <span class="name">${t.name}</span>
                    <span class="cat">${t.category || 'tool'}</span>
                    ${t.terminal && html`<span class="cat">terminal</span>`}
                    ${t.query && html`<span class="cat">info</span>`}
                  </div>
                  <p>${t.description}</p>
                  <details><summary class="muted">schema</summary>
                    <pre>${JSON.stringify(t.parameters || {}, null, 2)}</pre>
                  </details>
                </div>`)}
          </div>
        </div>
        <footer class="modal-foot">
          <p class="hint">Every model output is a tool call. Add a tool in Go with <code>registry.Register(...)</code>.</p>
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Close</button>
        </footer>
      </div>
    </div>`;
}

export function Toasts({ toasts }) {
  if (!toasts.length) return null;
  return html`
    <div class="toast-wrap">
      ${toasts.map((t) => html`
        <div class=${'toast' + (t.type === 'error' ? ' error' : '')} key=${t.id}
             dangerouslySetInnerHTML=${{ __html: formatText(t.message) }}></div>`)}
    </div>`;
}
