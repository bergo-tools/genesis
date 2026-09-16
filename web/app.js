import { h, Fragment, render } from './vendor/preact.module.js';
import { useCallback, useEffect, useRef, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { api } from './api.js';
import * as C from './components.js';

const html = htm.bind(h);
const LS_KEY = 'genesis.lastSession';

export function App() {
  const [config, setConfig] = useState(null);
  const [sessions, setSessions] = useState([]);
  const [session, setSession] = useState(null);
  const [status, setStatus] = useState(null);
  const [activity, setActivity] = useState([]);
  const [choices, setChoices] = useState([]);
  const [usage, setUsage] = useState(null);
  const [streaming, setStreaming] = useState(false);
  const [modal, setModal] = useState(null);
  const [tools, setTools] = useState([]);
  const [toasts, setToasts] = useState([]);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [panelOpen, setPanelOpen] = useState(false);

  const abortRef = useRef(null);
  const sessionRef = useRef(null);
  const toastId = useRef(0);

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  const pushToast = useCallback((message, type = 'info') => {
    const id = (toastId.current += 1);
    setToasts((list) => [...list, { id, message, type }]);
    setTimeout(() => setToasts((list) => list.filter((t) => t.id !== id)), type === 'error' ? 6500 : 3800);
  }, []);

  const refreshSessions = useCallback(async () => {
    try {
      const data = await api.sessions();
      setSessions(data.sessions || []);
    } catch (err) {
      setSessions([]);
    }
  }, []);

  const openSession = useCallback(async (id) => {
    try {
      const data = await api.session(id);
      setSession(data);
      setActivity([]);
      setChoices([]);
      setUsage(null);
      setStatus(null);
      localStorage.setItem(LS_KEY, id);
      setSidebarOpen(false);
      setPanelOpen(false);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast]);

  const handleEvent = useCallback((event) => {
    switch (event.type) {
      case 'status':
        setStatus({ status: event.status, step: event.step });
        break;
      case 'user_message':
        setSession((s) => {
          if (!s) return s;
          const list = s.messages.slice();
          const idx = list.findIndex((m) => m.pending);
          if (idx >= 0) list[idx] = event.message;
          else list.push(event.message);
          return { ...s, messages: list };
        });
        break;
      case 'title':
        setSession((s) => (s ? { ...s, title: event.title } : s));
        break;
      case 'message':
        setSession((s) => (s ? { ...s, messages: [...s.messages, event.message] } : s));
        break;
      case 'scene':
        setSession((s) => (s ? { ...s, scene: event.scene } : s));
        break;
      case 'character':
        setSession((s) => {
          if (!s) return s;
          const chars = (s.characters || []).slice();
          const idx = chars.findIndex((c) => c.id === event.character.id || c.name === event.character.name);
          if (idx >= 0) chars[idx] = event.character;
          else chars.push(event.character);
          return { ...s, characters: chars };
        });
        break;
      case 'memory':
        setSession((s) => (s ? { ...s, memories: [...(s.memories || []), event.memory] } : s));
        break;
      case 'state':
        setSession((s) => (s ? { ...s, state: event.state } : s));
        break;
      case 'choices':
        setChoices(event.choices || []);
        break;
      case 'reasoning':
        setActivity((list) => [...list, { id: 'think-' + list.length + '-' + Date.now(), name: 'think', done: true, result: event.text }]);
        break;
      case 'tool_start':
        setActivity((list) => [...list, {
          id: event.tool.id || event.tool.name + '-' + Date.now(),
          name: event.tool.name,
          args: event.tool.args ? JSON.stringify(event.tool.args, null, 2) : '',
          done: false,
        }]);
        break;
      case 'tool_end':
        setActivity((list) => {
          const next = list.slice();
          let idx = next.findIndex((a) => a.id === event.tool.id && !a.done);
          if (idx < 0) idx = next.findIndex((a) => a.name === event.tool.name && !a.done);
          const patch = {
            done: true,
            error: event.tool.error || '',
            result: event.tool.result ? JSON.stringify(event.tool.result, null, 2) : '',
          };
          if (idx >= 0) next[idx] = { ...next[idx], ...patch };
          else next.push({ id: event.tool.id || event.tool.name, name: event.tool.name, ...patch });
          return next;
        });
        break;
      case 'usage':
        setUsage(event.usage);
        break;
      case 'notice':
        pushToast(event.text);
        break;
      case 'error':
        pushToast(event.error, 'error');
        break;
      case 'turn_end':
        setStatus(null);
        break;
      default:
        break;
    }
  }, [pushToast]);

  const runStream = useCallback(async (path, body) => {
    if (abortRef.current) return;
    const controller = new AbortController();
    abortRef.current = controller;
    setStreaming(true);
    try {
      await api.stream(path, body, handleEvent, controller.signal);
    } catch (err) {
      if (err.name !== 'AbortError') pushToast(err.message, 'error');
    } finally {
      abortRef.current = null;
      setStreaming(false);
      setStatus(null);
      await refreshSessions();
    }
  }, [handleEvent, pushToast, refreshSessions]);

  const send = useCallback(async (text) => {
    const value = String(text || '').trim();
    const current = sessionRef.current;
    if (!value || !current) return;
    const pending = {
      id: 'pending-' + Date.now(),
      role: 'user',
      kind: 'user',
      speaker: (current.persona && current.persona.name) || '',
      text: value,
      pending: true,
      createdAt: new Date().toISOString(),
    };
    setSession((s) => (s ? { ...s, messages: [...s.messages, pending] } : s));
    setChoices([]);
    await runStream('/api/sessions/' + current.id + '/messages', { text: value });
  }, [runStream]);

  const saveConfig = useCallback(async (body) => {
    try {
      setConfig(await api.saveConfig(body));
      pushToast('Settings saved.');
      setModal(null);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast]);

  const loadModels = useCallback(async () => {
    try {
      const data = await api.models();
      return (data.models || []).map((m) => ({ id: m.id, name: m.name }));
    } catch (err) {
      pushToast(err.message, 'error');
      return [];
    }
  }, [pushToast]);

  const createStory = useCallback(async (body) => {
    try {
      const created = await api.createSession(body);
      setModal(null);
      setSession(created);
      setActivity([]);
      setChoices([]);
      setUsage(null);
      setStatus(null);
      localStorage.setItem(LS_KEY, created.id);
      setSidebarOpen(false);
      setPanelOpen(false);
      await refreshSessions();
      if (!created.messages || !created.messages.length) {
        await runStream('/api/sessions/' + created.id + '/opening', {});
      }
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast, refreshSessions, runStream]);

  const openTools = useCallback(async () => {
    setModal('tools');
    try {
      const data = await api.tools();
      setTools(data.tools || []);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast]);

  const openSettings = useCallback(async () => {
    setModal('settings');
    try {
      setConfig(await api.config());
    } catch (err) {
      /* keep the last known config */
    }
  }, []);

  const deleteCurrent = useCallback(async () => {
    const current = sessionRef.current;
    if (!current) return;
    if (!window.confirm('Delete this story? This cannot be undone.')) return;
    try {
      await api.deleteSession(current.id);
      setSession(null);
      localStorage.removeItem(LS_KEY);
      const list = (await api.sessions()).sessions || [];
      setSessions(list);
      if (list[0]) await openSession(list[0].id);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [openSession, pushToast]);

  useEffect(() => {
    (async () => {
      let cfg = null;
      try {
        cfg = await api.config();
        setConfig(cfg);
      } catch (err) {
        pushToast('Could not load configuration: ' + err.message, 'error');
      }
      let list = [];
      try {
        list = (await api.sessions()).sessions || [];
      } catch (err) {
        list = [];
      }
      setSessions(list);
      const last = localStorage.getItem(LS_KEY);
      const target = last && list.some((s) => s.id === last) ? last : list[0] && list[0].id;
      if (target) await openSession(target);
      if (cfg && !cfg.hasKey) pushToast('Add your OpenRouter API key in Settings to begin.');
    })();
  }, []);

  useEffect(() => {
    const onKey = (event) => {
      if ((event.metaKey || event.ctrlKey) && event.key === 'Delete') {
        event.preventDefault();
        deleteCurrent();
      }
      if (event.key === 'Escape') {
        setModal(null);
        setSidebarOpen(false);
        setPanelOpen(false);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [deleteCurrent]);

  const closeDrawers = () => {
    setSidebarOpen(false);
    setPanelOpen(false);
  };

  return html`
    <${Fragment}>
      <${C.Sidebar}
        sessions=${sessions}
        activeId=${session && session.id}
        open=${sidebarOpen}
        onOpen=${openSession}
        onNew=${() => setModal('new')}
        onSettings=${openSettings}
        onTools=${openTools}
      />
      ${(sidebarOpen || panelOpen) && html`
        <div class="scrim" onClick=${closeDrawers}></div>`}
      <main class="main">
        <${C.Topbar}
          session=${session}
          usage=${usage}
          onMenu=${() => setSidebarOpen((v) => !v)}
          onPanel=${() => setPanelOpen((v) => !v)}
        />
        <${C.SceneBar} scene=${session && session.scene} />
        <${C.MessageList} session=${session} streaming=${streaming} onNewStory=${() => setModal('new')} />
        <${C.StatusBar} status=${status && status.status} step=${status && status.step} />
        <${C.Choices} choices=${choices} onChoose=${send} />
        <${C.Composer}
          streaming=${streaming}
          disabled=${!session}
          onSend=${send}
          onStop=${() => { if (abortRef.current) abortRef.current.abort(); }}
        />
      </main>
      <${C.Panel}
        session=${session}
        activity=${activity}
        open=${panelOpen}
        onClose=${() => setPanelOpen(false)}
      />
      ${modal === 'settings' && html`
        <${C.SettingsModal}
          config=${config}
          onClose=${() => setModal(null)}
          onSave=${saveConfig}
          onLoadModels=${loadModels}
        />`}
      ${modal === 'new' && html`
        <${C.NewStoryModal} onClose=${() => setModal(null)} onCreate=${createStory} />`}
      ${modal === 'tools' && html`
        <${C.ToolsModal} tools=${tools} onClose=${() => setModal(null)} />`}
      <${C.Toasts} toasts=${toasts} />
    <//>`;
}

export function mount() {
  const root = document.getElementById('app');
  if (root) render(html`<${App} />`, root);
}

if (typeof document !== 'undefined') {
  mount();
}
