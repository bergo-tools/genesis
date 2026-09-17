import { h } from './vendor/preact.module.js';
import { useEffect, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { assetURL } from './api.js';
import { CharacterEditor } from './components.js';

const html = htm.bind(h);

export function StoryModal({ session, onClose, onSave, speechModel, onLoadSpeechModels }) {
  const s = session || {};
  const settings = s.settings || {};
  const [speechModels, setSpeechModels] = useState([]);
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
  const voiceOptions = ((speechModels.find((m) => m.id === (speechModel || '')) || {}).voices) || [];

  const [title, setTitle] = useState(s.title || '');
  const [avatar, setAvatar] = useState(s.avatar || '');
  const [avatarFile, setAvatarFile] = useState(null);
  const [avatarPreview, setAvatarPreview] = useState('');
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
  const pickStoryAvatar = (e) => {
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
        title,
        avatar,
        avatarFile,
        characters,
        settings: { systemPrompt },
      });
    } finally {
      setBusy(false);
    }
  };
  const src = avatarPreview || (avatar && s.id ? assetURL(s.id, avatar) : '');

  return html`
    <div class="modal" onClick=${ (e) => { if (e.target === e.currentTarget) onClose(); } }>
      <div class="modal-card">
        <header class="modal-head"><h2>Story settings</h2>
          <button class="icon-btn" type="button" onClick=${onClose}>×</button></header>
        <div class="modal-body">
          <div class="story-avatar-row">
            <label class="avatar-picker" title="Story avatar">
              ${src ? html`<img src=${src} alt="" />` : '+'}
              <input type="file" accept="image/*" onChange=${pickStoryAvatar} />
            </label>
            <label class="field grow"><span>Story title</span>
              <input type="text" value=${title} onInput=${ (e) => setTitle(e.currentTarget.value) } /></label>
          </div>

          <hr />
          <label class="field"><span>Story instructions</span>
            <textarea rows="3" placeholder="Extra instructions for this story only."
                      value=${systemPrompt} onInput=${ (e) => setSystemPrompt(e.currentTarget.value) }></textarea></label>
          <p class="hint">Model, temperature and tool switches are global — change them in Settings.</p>

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
                <${CharacterEditor} character=${c} index=${i} sessionId=${s.id} voiceOptions=${voiceOptions}
                  onChange=${update} onRemove=${remove} canRemove=${characters.length > 1} />
              </div>`)}
          </div>
        </div>
        <footer class="modal-foot">
          <button class="btn btn-ghost" type="button" onClick=${onClose}>Cancel</button>
          <button class="btn btn-primary" type="button" disabled=${busy} onClick=${save}>${busy ? 'Saving…' : 'Save'}</button>
        </footer>
      </div>
    </div>`;
}
