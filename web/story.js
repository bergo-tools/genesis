import { h } from './vendor/preact.module.js';
import { useEffect, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { assetURL } from './api.js';
import { CharacterEditor, ToolToggles, ReasoningSelect } from './components.js';
import { OptionPicker, modelOptions } from './picker.js';

const html = htm.bind(h);

export function StoryModal({ session, onClose, onSave, chatModels, speechModel, onLoadSpeechModels, tools }) {
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
  const [model, setModel] = useState(s.model || '');
  const [reasoningEffort, setReasoningEffort] = useState(settings.reasoningEffort || 'off');
  const [temperature, setTemperature] = useState(settings.temperature != null ? settings.temperature : 1);
  const [maxTokens, setMaxTokens] = useState(settings.maxTokens || 2048);
  const [maxSteps, setMaxSteps] = useState(settings.maxSteps || 6);
  const [systemPrompt, setSystemPrompt] = useState(settings.systemPrompt || '');
  const [choicesEnabled, setChoicesEnabled] = useState(settings.choicesEnabled !== false);
  const [disabledTools, setDisabledTools] = useState(settings.disabledTools || []);
  const [characters, setCharacters] = useState(() => (s.characters || []).map((c) => ({
    key: 'c' + Math.random().toString(36).slice(2),
    id: c.id || '', name: c.name || '', description: c.description || '',
    personality: c.personality || '', avatar: c.avatar || '', voice: c.voice || '',
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
  const pickModel = (id) => {
    setModel(id);
    const found = (chatModels || []).find((m) => m.id === id);
    if (found && found.maxOutput) setMaxTokens(found.maxOutput);
  };
  const save = async () => {
    setBusy(true);
    try {
      await onSave({
        title,
        avatar,
        avatarFile,
        model,
        characters,
        settings: {
          reasoningEffort,
          temperature: Number(temperature),
          maxTokens: Number(maxTokens),
          maxSteps: Number(maxSteps),
          systemPrompt,
          choicesEnabled,
          disabledTools,
        },
      });
    } finally {
      setBusy(false);
    }
  };
  const src = avatarPreview || (avatar && s.id ? assetURL(s.id, avatar) : '');
  const appliedModel = (chatModels || []).find((m) => m.id === model);
  const modelHint = appliedModel
    ? ('context ' + Math.round((appliedModel.context || 0) / 1000) + 'k · ' + (appliedModel.maxOutput
      ? 'max output ' + Math.round(appliedModel.maxOutput / 1000) + 'k'
      : 'provider does not report a max output'))
    : ((chatModels || []).length + ' models available');

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
          <strong>Model &amp; generation</strong>
          <div class="row">
            <div class="field grow"><span>Model</span>
              <${OptionPicker} value=${model} options=${modelOptions(chatModels)} onChange=${pickModel}
                placeholder="Select a model" title="Chat model" /></div>
          </div>
          <span class="hint">${modelHint}</span>
          <${ReasoningSelect} value=${reasoningEffort} onChange=${setReasoningEffort} />
          <div class="row">
            <label class="field"><span>Temperature</span>
              <input type="number" min="0" max="2" step="0.05" value=${temperature}
                     onInput=${ (e) => setTemperature(e.currentTarget.value) } /></label>
            <label class="field"><span>Max output tokens</span>
              <input type="number" min="64" step="64" value=${maxTokens}
                     onInput=${ (e) => setMaxTokens(e.currentTarget.value) } /></label>
            <label class="field"><span>Max steps</span>
              <input type="number" min="1" max="40" value=${maxSteps}
                     onInput=${ (e) => setMaxSteps(e.currentTarget.value) } /></label>
          </div>
          <label class="field"><span>Story instructions</span>
            <textarea rows="3" placeholder="Extra instructions for this story only."
                      value=${systemPrompt} onInput=${ (e) => setSystemPrompt(e.currentTarget.value) }></textarea></label>

          <hr />
          <strong>Tools</strong>
          <${ToolToggles}
            tools=${tools}
            choicesEnabled=${choicesEnabled}
            disabledTools=${disabledTools}
            onChoices=${setChoicesEnabled}
            onToggle=${ (name, on) => setDisabledTools((cur) => (on ? cur.filter((n) => n !== name) : cur.concat([name]))) } />

          <hr />
          <div>
            <div class="row" style="justify-content:space-between;align-items:center">
              <strong>Cast</strong>
              <button class="btn btn-ghost btn-sm" type="button"
                      onClick=${ () => setCharacters((cur) => cur.concat([{
                        key: 'c' + Math.random().toString(36).slice(2), id: '', name: '', description: '',
                        personality: '', avatar: '', voice: '', avatarFile: null, avatarPreview: '',
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
