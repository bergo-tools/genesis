import { h } from './vendor/preact.module.js';
import { useEffect, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { assetURL, storyAssetURL } from './api.js';
import { CharacterEditor } from './components.js';

const html = htm.bind(h);

// preview trims a long text (e.g. a preset opening) for a compact hint.
function preview(text, max) {
  const value = String(text || '').trim();
  return value.length > max ? value.slice(0, max) + '…' : value;
}

// StoryAvatar is always square-cropped: an unconstrained .avatar would let a
// large upload render at its intrinsic size inside the preset lists.
function StoryAvatar({ story, size = 40 }) {
  const src = story && story.avatar ? storyAssetURL(story.id, story.avatar) : '';
  return html`
    <div class="avatar" style=${'width:' + size + 'px;height:' + size + 'px'}>
      ${src ? html`<img src=${src} alt="" />` : ((story && story.title) || '?').trim().charAt(0)}
    </div>`;
}

export function NewSessionModal({ stories, onClose, onCreate }) {
  const list = stories || [];
  const [storyId, setStoryId] = useState(list.length ? list[0].id : '');
  const [title, setTitle] = useState('');
  const [personaName, setPersonaName] = useState('');
  const [personaDesc, setPersonaDesc] = useState('');
  const [busy, setBusy] = useState(false);
  const selected = list.find((s) => s.id === storyId);
  const create = async () => {
    if (!storyId) return;
    setBusy(true);
    try {
      await onCreate({ storyId, title, persona: { name: personaName, description: personaDesc } });
    } finally {
      setBusy(false);
    }
  };
  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>New session</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
        <div class="modal-body">
          ${!list.length
            ? html`<p class="muted">No presets yet. Create one under Stories first.</p>`
            : html`
              <div>
                <strong>Pick a story</strong>
                <div class="preset-list">
                  ${list.map((s) => html`
                    <button type="button" key=${s.id}
                            class=${'preset-card' + (s.id === storyId ? ' active' : '')}
                            onClick=${ () => setStoryId(s.id) }>
                      <${StoryAvatar} story=${s} />
                      <div class="preset-meta">
                        <div class="name">${s.title}</div>
                        <div class="desc">${s.description || ''}</div>
                        <div class="hint">${(s.characterCount || 0) + ' characters' + (s.genre ? ' · ' + s.genre : '')}</div>
                      </div>
                    </button>`)}
                </div>
              </div>`}
          ${selected && selected.opening ? html`<p class="hint">${preview(selected.opening, 220)}</p>` : null}
          <label class="field"><span>Session title (optional)</span>
            <input type="text" placeholder=${selected ? selected.title : 'Optional'} value=${title}
                   onInput=${ (e) => setTitle(e.currentTarget.value) } /></label>
          <div class="grid-2">
            <label class="field"><span>Your name</span>
              <input type="text" placeholder="The player" value=${personaName}
                     onInput=${ (e) => setPersonaName(e.currentTarget.value) } /></label>
            <label class="field"><span>Your description</span>
              <input type="text" placeholder="optional" value=${personaDesc}
                     onInput=${ (e) => setPersonaDesc(e.currentTarget.value) } /></label>
          </div>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" disabled=${busy || !storyId} onClick=${create}>${busy ? 'Starting…' : 'Start'}</button>
        </footer>
      </div>
    </div>`;
}

export function StoriesModal({ stories, onClose, onStart, onEdit, onNew, onDelete }) {
  const list = stories || [];
  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>Stories (presets)</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
        <div class="modal-body">
          <div class="row" style="justify-content:space-between;align-items:center">
            <span class="hint">${list.length + ' presets · every session starts from one'}</span>
            <button class="btn btn-primary btn-sm" type="button" onClick=${onNew}>+ New preset</button>
          </div>
          <div class="preset-list">
            ${!list.length && html`<p class="muted">No presets yet.</p>`}
            ${list.map((s) => html`
              <div class="preset-row" key=${s.id}>
                <${StoryAvatar} story=${s} />
                <div class="preset-meta">
                  <div class="name">${s.title}${s.builtin ? ' · builtin' : ''}</div>
                  <div class="desc">${s.description || ''}</div>
                  <div class="hint">${(s.characterCount || 0) + ' characters' + (s.genre ? ' · ' + s.genre : '')}</div>
                </div>
                <div class="preset-actions">
                  <button class="btn btn-primary btn-sm" type="button" onClick=${ () => onStart(s.id) }>Start</button>
                  <button class="btn btn-ghost btn-sm" type="button" onClick=${ () => onEdit(s.id) }>Edit</button>
                  <button class="btn btn-ghost btn-sm" type="button" onClick=${ () => onDelete(s) }>Delete</button>
                </div>
              </div>`)}
          </div>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Close</button>
        </footer>
      </div>
    </div>`;
}

// A preset is content only. Model and generation options come from the global
// config (snapshotted into a session when it starts), never from a preset.
export function PresetModal({ story, onClose, onSave }) {
  const s = story || {};
  const settings = s.settings || {};
  const [title, setTitle] = useState(s.title || '');
  const [avatar, setAvatar] = useState(s.avatar || '');
  const [avatarFile, setAvatarFile] = useState(null);
  const [avatarPreview, setAvatarPreview] = useState('');
  const [description, setDescription] = useState(s.description || '');
  const [genre, setGenre] = useState(s.genre || '');
  const [opening, setOpening] = useState(s.opening || '');
  const [personaDesc, setPersonaDesc] = useState((s.persona && s.persona.description) || '');
  const [systemPrompt, setSystemPrompt] = useState(settings.systemPrompt || '');
  const [characters, setCharacters] = useState(() => (s.characters || []).map((c) => ({
    key: 'c' + Math.random().toString(36).slice(2),
    id: c.id || '', name: c.name || '', description: c.description || '',
    avatar: c.avatar || '', voice: c.voice || '',
    avatarFile: null, avatarPreview: '',
  })));
  const [busy, setBusy] = useState(false);

  const update = (index, patch) => setCharacters((cur) => cur.map((c, i) => (i === index ? { ...c, ...patch } : c)));
  const remove = (index) => setCharacters((cur) => cur.filter((_, i) => i !== index));
  const pickAvatar = (e) => {
    const file = e.currentTarget.files && e.currentTarget.files[0];
    if (file) {
      setAvatarFile(file);
      setAvatarPreview(URL.createObjectURL(file));
    }
  };
  const save = async () => {
    setBusy(true);
    try {
      await onSave({
        id: s.id || '',
        title,
        avatar,
        avatarFile,
        description,
        genre,
        opening,
        persona: { name: (s.persona && s.persona.name) || '', description: personaDesc },
        characters,
        settings: { systemPrompt },
      });
    } finally {
      setBusy(false);
    }
  };
  const src = avatarPreview || (avatar && s.id ? storyAssetURL(s.id, avatar) : '');
  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>${s.id ? 'Edit preset' : 'New preset'}</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
        <div class="modal-body">
          <div class="story-avatar-row">
            <label class="avatar-picker" title="Story avatar">
              ${src ? html`<img src=${src} alt="" />` : '+'}
              <input type="file" accept="image/*" onChange=${pickAvatar} />
            </label>
            <label class="field grow"><span>Title</span>
              <input type="text" placeholder="Emberfall" value=${title}
                     onInput=${ (e) => setTitle(e.currentTarget.value) } /></label>
          </div>
          <label class="field"><span>Genre</span>
            <input type="text" placeholder="dark fantasy" value=${genre}
                   onInput=${ (e) => setGenre(e.currentTarget.value) } /></label>
          <label class="field"><span>Description</span>
            <textarea rows="2" value=${description} onInput=${ (e) => setDescription(e.currentTarget.value) }></textarea></label>
          <label class="field"><span>Opening message</span>
            <textarea rows="3" placeholder="Shown when a session starts; leave blank to let the agent open."
                      value=${opening} onInput=${ (e) => setOpening(e.currentTarget.value) }></textarea></label>
          <label class="field"><span>Suggested player description</span>
            <input type="text" value=${personaDesc} onInput=${ (e) => setPersonaDesc(e.currentTarget.value) } /></label>

          <label class="field"><span>Story instructions</span>
            <textarea rows="3" placeholder="Extra instructions for this story only."
                      value=${systemPrompt} onInput=${ (e) => setSystemPrompt(e.currentTarget.value) }></textarea></label>

          <hr />
          <div>
            <div class="row" style="justify-content:space-between;align-items:center">
              <strong>Cast</strong>
              <button class="btn btn-ghost btn-sm" type="button"
                      onClick=${ () => setCharacters((cur) => cur.concat([{
                        key: 'c' + Math.random().toString(36).slice(2), id: '', name: '', description: '',
                        avatar: '', voice: '', avatarFile: null, avatarPreview: '',
                      }])) }>+ Add character</button>
            </div>
            ${characters.map((c, i) => html`
              <div key=${c.key} style="margin-top:10px">
                <${CharacterEditor} character=${c} index=${i} sessionId=${''}
                  onChange=${update} onRemove=${remove} canRemove=${characters.length > 1} />
              </div>`)}
          </div>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" disabled=${busy || !title.trim()} onClick=${save}>${busy ? 'Saving…' : 'Save'}</button>
        </footer>
      </div>
    </div>`;
}
