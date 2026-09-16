import { h } from './vendor/preact.module.js';
import { useEffect, useRef, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { assetURL } from './api.js';
import { formatText, initial } from './format.js';
import { TokenStatus, TokenPanel } from './tokens.js';
import { OptionPicker, modelOptions, speechOptions, stringOptions } from './picker.js';

const html = htm.bind(h);

const REASONING_LEVELS = [
  ['off', 'off (no thinking)'],
  ['minimal', 'minimal'],
  ['low', 'low'],
  ['medium', 'medium'],
  ['high', 'high'],
  ['max', 'max'],
];

function avatarFor(message, session) {
  if (message.role === 'user') {
    const name = session && session.persona && session.persona.name;
    return name ? initial(name) : '🧍';
  }
  const chars = (session && session.characters) || [];
  const speaker = String(message.speaker || '').toLowerCase();
  const found = chars.find((c) => String(c.name || '').toLowerCase() === speaker);
  if (found && found.avatar && session) return html`<img src=${assetURL(session.id, found.avatar)} alt="" />`;
  if (found && found.name) return initial(found.name);
  switch (message.kind) {
    case 'narration': return '✦';
    case 'prompt': return '❯';
    case 'thought': return '…';
    default: return '◆';
  }
}

function messageImages(message, session) {
  if (message.images && message.images.length && session) {
    return message.images.map((name) => assetURL(session.id, name)).filter(Boolean);
  }
  return message.localImages || [];
}

export function Message({ message, session, onSpeak, speaking, action, onReroll, onEdit, busy }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(message.text || '');
  const speakable = Boolean(message.text) && message.kind !== 'scene';
  const canEdit = Boolean(action && action.editable);
  const canReroll = Boolean(action && action.rerollFrom);
  const hasActions = speakable || canEdit || canReroll;
  const startEdit = () => {
    setDraft(message.text || '');
    setEditing(true);
  };
  const saveEdit = () => {
    setEditing(false);
    if (onEdit) onEdit(message, draft);
  };
  const cls = ['msg', message.role, message.kind, message.pending ? 'pending' : ''].filter(Boolean).join(' ');
  const speaker = message.speaker || (message.role === 'user'
    ? ((session && session.persona && session.persona.name) || 'You')
    : '');
  const showSpeaker = speaker && message.kind !== 'narration';
  const images = messageImages(message, session);
  return html`
    <div class=${cls} data-id=${message.id}>
      <div class="avatar">${avatarFor(message, session)}</div>
      <div class="body">
        ${(showSpeaker || message.mood) && html`
          <div class="speaker">
            ${showSpeaker && html`<span>${speaker}</span>`}
            ${message.mood && html`<span class="mood">${message.mood}</span>`}
          </div>`}
        ${images.length > 0 && html`
          <div class="attachments">
            ${images.map((src, i) => html`<img class="attachment" key=${i} src=${src} alt="attachment" loading="lazy" />`)}
          </div>`}
        ${message.ooc ? html`
          <div class="ooc-inline"><span class="ooc-tag">OOC</span>
            <span dangerouslySetInnerHTML=${{ __html: formatText(message.ooc) }}></span></div>` : null}
        ${editing ? html`
          <textarea class="edit-box" rows="3" value=${draft}
                    onInput=${(e) => setDraft(e.currentTarget.value)}></textarea>
          <div class="msg-actions">
            <button class="btn btn-primary btn-sm" type="button" disabled=${busy} onClick=${saveEdit}>${busy ? '…' : 'Save & rerun'}</button>
            <button class="btn btn-ghost btn-sm" type="button" onClick=${() => setEditing(false)}>Cancel</button>
          </div>`
        : html`
          ${message.text ? html`
            <div class="text" dangerouslySetInnerHTML=${{ __html: formatText(message.text) }}></div>` : null}
          ${message.thought ? html`
            <div class="thought-inline" dangerouslySetInnerHTML=${{ __html: formatText(message.thought) }}></div>` : null}
          ${hasActions ? html`
            <div class="msg-actions">
              ${speakable && html`
                <button class=${'speak-btn' + (speaking ? ' active' : '')} type="button"
                        title=${speaking ? 'Stop' : 'Read aloud'}
                        onClick=${() => onSpeak && onSpeak(message)}>${speaking ? '⏹' : '🔊'}</button>`}
              ${canEdit && html`
                <button class="speak-btn" type="button" title="Edit and rerun from here" onClick=${startEdit}>✎</button>`}
              ${canReroll && html`
                <button class="speak-btn" type="button" title="Re-roll this turn"
                        onClick=${() => onReroll && onReroll(action.rerollFrom)}>↻</button>`}
            </div>` : null}`}
      </div>
    </div>`;
}

export function MessageList({ session, streaming, onNewStory, onSpeak, speakingId, onReroll, onEdit, busy }) {
  const ref = useRef(null);
  const messages = (session && session.messages) || [];
  // Per-turn actions: a user message is editable, and the last assistant
  // message of its turn gets the re-roll button.
  const actions = {};
  for (let i = 0; i < messages.length; i++) {
    if (messages[i].kind !== 'user') continue;
    let j = i + 1;
    while (j < messages.length && messages[j].kind !== 'user') j++;
    if (j - 1 > i) {
      const lastID = messages[j - 1].id;
      actions[lastID] = Object.assign({}, actions[lastID], { rerollFrom: messages[i].id });
    }
    actions[messages[i].id] = Object.assign({}, actions[messages[i].id], { editable: true });
  }
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
          <p>Genesis is an agentic game master. Every beat is a tool call: the cast speaks and thinks
             with message, sets the scene with scene, and hands you the next branches with choices.</p>
          <p><button class="btn btn-primary" type="button" onClick=${onNewStory}>Create your first story</button></p>
        </div>
      </section>`;
  }
  return html`
    <section class="messages" ref=${ref}>
      ${messages.map((m) => html`<${Message} key=${m.id} message=${m} session=${session} onSpeak=${onSpeak} speaking=${speakingId === m.id} action=${actions[m.id]} onReroll=${onReroll} onEdit=${onEdit} busy=${busy} />`)}
    </section>`;
}

export function SceneBar({ scene }) {
  const s = scene || {};
  const fields = [['📍', s.location], ['🕓', s.time], ['☁', s.weather]]
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
      <span class="choices-label">Choose a path, or write your own below:</span>
      ${choices.map((choice, index) => html`
        <button key=${index} type="button" class="choice" onClick=${() => onChoose(choice.text)}>
          ${choice.text}
          ${choice.description && html`<small>${choice.description}</small>`}
        </button>`)}
    </section>`;
}

export function Composer({ streaming, disabled, uploading, hasChoices, onSend, onStop }) {
  const ref = useRef(null);
  const fileRef = useRef(null);
  const [value, setValue] = useState('');
  const [ooc, setOoc] = useState('');
  const [showOoc, setShowOoc] = useState(false);
  const [files, setFiles] = useState([]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, Math.round(window.innerHeight * 0.4)) + 'px';
  }, [value]);

  const busy = streaming || uploading;
  const locked = Boolean(disabled);
  const submit = () => {
    const text = value.trim();
    const directive = ooc.trim();
    if (locked || busy || (!text && !files.length && !directive)) return;
    const payload = files.map((f) => f.file);
    setValue('');
    setOoc('');
    if (!directive) setShowOoc(false);
    setFiles([]);
    onSend(text, payload, directive);
  };
  const addFiles = (e) => {
    const picked = Array.from(e.currentTarget.files || []);
    if (picked.length) {
      setFiles((cur) => cur.concat(picked.map((file) => ({ file, url: URL.createObjectURL(file) }))));
    }
    e.currentTarget.value = '';
  };
  const removeFile = (index) => setFiles((cur) => cur.filter((_, i) => i !== index));

  return html`
    <footer class="composer">
      ${files.length > 0 && html`
        <div class="composer-attachments">
          ${files.map((f, i) => html`
            <div class="attachment-chip" key=${i}>
              <img src=${f.url} alt="" />
              <button type="button" title="Remove" onClick=${() => removeFile(i)}>×</button>
            </div>`)}
        </div>`}
      ${showOoc && html`
        <textarea class="ooc-box" rows="2" value=${ooc} autocomplete="off" disabled=${locked}
                  placeholder="Out-of-character instruction to the model — it directs the story, it is not part of it."
                  onInput=${ (e) => setOoc(e.currentTarget.value) }></textarea>`}
      <div class="composer-row">
        <button class="btn btn-ghost attach-btn" type="button" title="Attach an image"
                disabled=${locked || busy} onClick=${() => fileRef.current && fileRef.current.click()}>🖼</button>
        <input ref=${fileRef} type="file" accept="image/*" multiple hidden disabled=${locked || busy} onChange=${addFiles} />
        <textarea
          ref=${ref}
          rows="1"
          value=${value}
          autocomplete="off"
          disabled=${locked}
          placeholder=${locked ? 'Create a story to start…' : (hasChoices ? 'Or write your own action…' : 'What do you do?')}
          onInput=${ (e) => setValue(e.currentTarget.value) }
          onKeyDown=${ (e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); submit(); } } }
        ></textarea>
        <button class=${'btn btn-ghost btn-ooc' + (showOoc ? ' active' : '')} type="button"
                title="Add an out-of-character instruction" disabled=${locked} onClick=${() => setShowOoc((v) => !v)}>OOC</button>
        ${streaming
          ? html`<button class="btn btn-danger" type="button" onClick=${onStop}>Cancel</button>`
          : html`<button class="btn btn-primary" type="button" disabled=${Boolean(disabled) || uploading} onClick=${submit}>${uploading ? '…' : 'Send'}</button>`}
      </div>
    </footer>`;
}

export function Sidebar({ sessions, activeId, open, auth, onOpen, onNew, onSettings, onTools, onStories, onDelete, onLogout }) {
  return html`
    <aside class=${'sidebar' + (open ? ' open' : '')} aria-label="Stories">
      <div class="sidebar-head">
        <span class="brand"><span class="brand-mark">◆</span> Genesis</span>
        <button class="btn btn-primary btn-sm" type="button" onClick=${onNew}>New story</button>
      </div>
      <nav class="session-list">
        ${!sessions.length && html`<p class="muted" style="padding:10px">No stories yet.</p>`}
        ${sessions.map((s) => html`
          <div key=${s.id} class=${'session-item' + (s.id === activeId ? ' active' : '')}>
            <button type="button" class="session-open" onClick=${() => onOpen(s.id)}>
              <span class="t">${s.title || 'Untitled'}</span>
              <span class="s">${((s.characters || []).join(', ') || '') + (s.messageCount ? ' · ' + s.messageCount + ' msg' : '')}</span>
            </button>
            <button type="button" class="session-del" title="Delete session"
                    onClick=${(e) => { e.stopPropagation(); onDelete && onDelete(s); }}>✕</button>
          </div>`)}
      </nav>
      <div class="sidebar-foot">
        <button class="btn btn-ghost btn-sm" type="button" onClick=${onStories}>Stories</button>
        <button class="btn btn-ghost btn-sm" type="button" onClick=${onTools}>Tools</button>
        <button class="btn btn-ghost btn-sm" type="button" onClick=${onSettings}>Settings</button>
        ${auth && auth.enabled && onLogout && html`
          <button class="btn btn-ghost btn-sm" type="button" title="Sign out" onClick=${onLogout}>Log out</button>`}
      </div>
    </aside>`;
}

export function Topbar({ session, models, model, onMenu, onPanel }) {
  const bits = [];
  if (session && session.storyTitle) bits.push(session.storyTitle);
  if (model) bits.push(model);
  if (session && session.characters && session.characters.length) bits.push(session.characters.map((c) => c.name).join(', '));
  return html`
    <header class="topbar">
      <button id="menu-toggle" class="icon-btn" type="button" aria-label="Toggle stories" onClick=${onMenu}>☰</button>
      ${session && session.avatar
        ? html`<img class="story-avatar" src=${assetURL(session.id, session.avatar)} alt="" />`
        : null}
      <div class="topbar-title">
        <span id="chat-title">${(session && session.title) || 'Genesis'}</span>
        <small id="chat-subtitle">${bits.join(' · ') || 'agentic roleplay'}</small>
      </div>
      <${TokenStatus} session=${session} models=${models} model=${model} />
      <button class="icon-btn" type="button" aria-label="Toggle world panel" onClick=${onPanel}>◧</button>
    </header>`;
}

export function Panel({ session, activity, open, models, model, onClose, onEditCast }) {
  const chars = (session && session.characters) || [];
  return html`
    <aside class=${'panel' + (open ? ' open' : '')} aria-label="World state">
      <div class="panel-head">
        <span>World</span>
        <button class="icon-btn" type="button" aria-label="Close" onClick=${onClose}>×</button>
      </div>
      <div class="panel-body">
        <${TokenPanel} session=${session} models=${models} model=${model} />
        <section class="panel-section">
          <div class="panel-head" style="padding:0;border:none">
            <h3 style="margin:0">Cast</h3>
            <button class="btn btn-ghost btn-sm" type="button" onClick=${onEditCast} disabled=${!session}>Edit cast</button>
          </div>
          ${!chars.length
            ? html`<p class="muted">No characters yet.</p>`
            : chars.map((c) => html`
              <div class="char-card" key=${c.id || c.name}>
                <div class="avatar">
                  ${c.avatar && session ? html`<img src=${assetURL(session.id, c.avatar)} alt="" />` : initial(c.name)}
                </div>
                <div>
                  <div class="name">${c.name || 'Unnamed'}</div>
                  <div class="desc">${c.description || c.personality || ''}</div>
                </div>
              </div>`)}
        </section>
        <section class="panel-section">
          <h3>World state</h3>
          <pre class="state-json">${JSON.stringify((session && session.state) || {}, null, 2)}</pre>
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

export function CharacterEditor({ character, index, sessionId, onChange, onRemove, canRemove, voiceOptions }) {
  const set = (key) => (e) => onChange(index, { [key]: e.currentTarget.value });
  const pick = (e) => {
    const file = e.currentTarget.files && e.currentTarget.files[0];
    if (file) onChange(index, { avatarFile: file, avatarPreview: URL.createObjectURL(file) });
  };
  const src = character.avatarPreview || (character.avatar && sessionId ? assetURL(sessionId, character.avatar) : '');
  return html`
    <div class="char-editor">
      <div class="char-editor-head">
        <span class="char-editor-title">Character ${index + 1}</span>
        ${canRemove && html`<button class="btn btn-ghost btn-sm" type="button" onClick=${() => onRemove(index)}>Remove</button>`}
      </div>
      <div class="char-editor-body">
        <label class="avatar-picker" title="Upload avatar">
          ${src ? html`<img src=${src} alt="" />` : (character.name ? initial(character.name) : '+')}
          <input type="file" accept="image/*" onChange=${pick} />
        </label>
        <div class="char-editor-fields">
          <label class="field"><span>Name</span>
            <input type="text" value=${character.name} placeholder="Ilyra" onInput=${set('name')} /></label>
          <label class="field"><span>Description</span>
            <input type="text" value=${character.description} placeholder="An archivist who guards a drowned library."
                   onInput=${set('description')} /></label>
          <label class="field"><span>Personality</span>
            <input type="text" value=${character.personality} placeholder="Dry, patient, secretly sentimental."
                   onInput=${set('personality')} /></label>
          <div class="field"><span>Voice (TTS)</span>
            <${OptionPicker} value=${character.voice} options=${stringOptions(voiceOptions)}
              onChange=${ (v) => onChange(index, { voice: v }) }
              placeholder="af_heart" title="Voice" /></div>
        </div>
      </div>
    </div>`;
}

export function ReasoningSelect({ value, onChange }) {
  return html`
    <label class="field"><span>Thinking effort</span>
      <select value=${value} onChange=${ (e) => onChange(e.currentTarget.value) }>
        ${REASONING_LEVELS.map(([id, label]) => html`<option key=${id} value=${id}>${label}</option>`)}
      </select>
    </label>`;
}

export function ToolToggles({ tools, choicesEnabled, disabledTools, onChoices, onToggle }) {
  const list = tools || [];
  if (!list.length) {
    return html`<p class="muted">No tools registered.</p>`;
  }
  return html`
    <div class="tool-toggles">
      ${list.map((t) => {
        const isChoices = t.name === 'choices';
        const enabled = isChoices ? choicesEnabled : !(disabledTools || []).includes(t.name);
        const change = (e) => {
          const on = e.currentTarget.checked;
          if (isChoices) onChoices(on);
          else onToggle(t.name, on);
        };
        return html`
          <label class="toggle-row" key=${t.name}>
            <input type="checkbox" checked=${enabled} onChange=${change} />
            <span class="toggle-name">${t.name}</span>
            <span class="toggle-desc">${isChoices ? 'ends every turn; required when enabled' : t.description}</span>
          </label>`;
      })}
    </div>`;
}

export function SettingsModal({ config, onClose, onSave, chatModels, onReloadModels, onLoadSpeechModels, tools }) {
  const cfg = config || {};
  const [form, setForm] = useState({
    apiKey: '',
    baseUrl: cfg.baseUrl || '',
    model: cfg.model || '',
    temperature: cfg.temperature != null ? cfg.temperature : 1,
    maxTokens: cfg.maxTokens || 2048,
    maxSteps: cfg.maxSteps || 6,
    toolChoice: cfg.toolChoice || 'auto',
    systemPrompt: cfg.systemPrompt || '',
    reasoningEffort: cfg.reasoningEffort || 'off',
    choicesEnabled: cfg.choicesEnabled !== false,
    speechModel: cfg.speechModel || '',
    speechVoice: cfg.speechVoice || '',
    speechFormat: cfg.speechFormat || 'mp3',
    speechSpeed: cfg.speechSpeed != null ? cfg.speechSpeed : 1,
    autoSpeak: cfg.autoSpeak === true,
    disabledTools: cfg.disabledTools || [],
  });
  const [speechModels, setSpeechModels] = useState([]);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let alive = true;
    if (onLoadSpeechModels) {
      onLoadSpeechModels().then((list) => {
        if (alive) setSpeechModels(list || []);
      });
    }
    return () => {
      alive = false;
    };
  }, [onLoadSpeechModels]);
  const voiceOptions = ((speechModels.find((m) => m.id === form.speechModel) || {}).voices) || [];
  const set = (key) => (e) => setForm((f) => ({ ...f, [key]: e.currentTarget.value }));
  const applyModel = (id) => {
    const found = (chatModels || []).find((m) => m.id === id);
    setForm((f) => ({ ...f, model: id, maxTokens: found && found.maxOutput ? found.maxOutput : f.maxTokens }));
  };
  const currentModel = (chatModels || []).find((m) => m.id === form.model);
  const modelHint = currentModel
    ? 'context ' + Math.round((currentModel.context || 0) / 1000) + 'k · ' + (currentModel.maxOutput
      ? 'max output ' + Math.round(currentModel.maxOutput / 1000) + 'k'
      : 'provider does not report a max output')
    : (chatModels || []).length + ' models · ' + (chatModels || []).filter((m) => m.tools).length + ' support tools';
  const reload = async () => {
    setBusy(true);
    try {
      await onReloadModels();
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
      reasoningEffort: form.reasoningEffort,
      choicesEnabled: form.choicesEnabled,
      speechModel: form.speechModel.trim(),
      speechVoice: form.speechVoice.trim(),
      speechFormat: form.speechFormat,
      speechSpeed: Number(form.speechSpeed),
      autoSpeak: form.autoSpeak,
      disabledTools: form.disabledTools,
    };
    if (form.apiKey.trim()) body.apiKey = form.apiKey.trim();
    onSave(body);
  };
  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>Settings</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
        <div class="modal-body">
          <label class="field"><span>API key</span>
            <input type="password" placeholder=${cfg.hasKey ? '•••••••• (leave blank to keep)' : 'sk-or-...'}
                   value=${form.apiKey} onInput=${set('apiKey')} /></label>
          <p class="hint">${cfg.hasKey ? 'A key is configured. Leave blank to keep it.' : 'No key configured yet.'}</p>
          <label class="field"><span>Base URL</span><input type="text" value=${form.baseUrl} onInput=${set('baseUrl')} /></label>
          <div class="row">
            <div class="field grow"><span>Model</span>
              <${OptionPicker} value=${form.model} options=${modelOptions(chatModels)} onChange=${applyModel}
                placeholder="Select a model" title="Chat model" /></div>
            <button class="btn btn-ghost btn-sm" type="button" disabled=${busy} onClick=${reload}>${busy ? '…' : 'Reload'}</button>
          </div>
          <span class="hint">${modelHint}</span>
          <div class="row">
            <label class="field"><span>Temperature</span>
              <input type="number" min="0" max="2" step="0.05" value=${form.temperature} onInput=${set('temperature')} /></label>
            <label class="field"><span>Max tokens</span>
              <input type="number" min="64" step="64" value=${form.maxTokens} onInput=${set('maxTokens')} /></label>
            <label class="field"><span>Max steps</span>
              <input type="number" min="1" max="40" value=${form.maxSteps} onInput=${set('maxSteps')} /></label>
          </div>
          <${ReasoningSelect} value=${form.reasoningEffort} onChange=${ (v) => setForm((f) => ({ ...f, reasoningEffort: v })) } />
          <hr />
          <strong>Tools</strong>
          <${ToolToggles}
            tools=${tools}
            choicesEnabled=${form.choicesEnabled}
            disabledTools=${form.disabledTools}
            onChoices=${ (on) => setForm((f) => ({ ...f, choicesEnabled: on })) }
            onToggle=${ (name, on) => setForm((f) => ({
              ...f,
              disabledTools: on ? f.disabledTools.filter((n) => n !== name) : f.disabledTools.concat([name]),
            })) } />
          <hr />
          <strong>Text to speech</strong>
          <div class="grid-2">
            <div class="field"><span>Speech model</span>
              <${OptionPicker} value=${form.speechModel} options=${speechOptions(speechModels)}
                onChange=${ (v) => setForm((f) => ({ ...f, speechModel: v })) }
                placeholder="hexgrad/kokoro-82m" title="Speech model" />
              <span class="hint">${speechModels.length + ' TTS models'}</span></div>
            <div class="field"><span>Default voice</span>
              <${OptionPicker} value=${form.speechVoice} options=${stringOptions(voiceOptions)}
                onChange=${ (v) => setForm((f) => ({ ...f, speechVoice: v })) }
                placeholder="af_heart" title="Voice" />
              <span class="hint">${speechModels.length === 0
                ? 'Voice catalog unavailable — type a voice your model supports.'
                : voiceOptions.length + ' voices for this model'}</span></div>
          </div>
          <div class="row">
            <label class="field"><span>Format</span>
              <select value=${form.speechFormat} onChange=${set('speechFormat')}>
                <option value="mp3">mp3</option>
                <option value="wav">wav</option>
                <option value="opus">opus</option>
                <option value="aac">aac</option>
                <option value="flac">flac</option>
                <option value="pcm">pcm</option>
              </select></label>
            <label class="field"><span>Speed</span>
              <input type="number" min="0.25" max="4" step="0.05" value=${form.speechSpeed} onInput=${set('speechSpeed')} /></label>
          </div>
          <label class="switch">
            <input type="checkbox" checked=${form.autoSpeak}
                   onChange=${ (e) => setForm((f) => ({ ...f, autoSpeak: e.currentTarget.checked })) } />
            Read new messages aloud automatically
          </label>
          <label class="field"><span>Tool choice</span>
            <select value=${form.toolChoice} onChange=${set('toolChoice')}>
              <option value="auto">auto</option>
              <option value="required">required (force a tool call)</option>
              <option value="none">none</option>
            </select></label>
          <label class="field"><span>Global director instructions</span>
            <textarea rows="4" placeholder="Extra standing instructions appended to every system prompt."
                      value=${form.systemPrompt} onInput=${set('systemPrompt')}></textarea></label>
          <p class="hint">Model and generation settings are global: saving applies them to every story, including the one you have open.</p>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" onClick=${save}>Save</button>
        </footer>
      </div>
    </div>`;
}

export function ToolsModal({ tools, onClose }) {
  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>Registered tools</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
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
