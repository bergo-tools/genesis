import { h, Fragment, render } from './vendor/preact.module.js';
import { useCallback, useEffect, useRef, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { api, uploadAsset, setUnauthorizedHandler } from './api.js';
import * as C from './components.js';
import { StoryModal } from './story.js';
import { NewSessionModal, StoriesModal, PresetModal } from './library.js';
import { Login } from './login.js';

const html = htm.bind(h);
const LS_KEY = 'genesis.lastSession';
const LS_SCENE = 'genesis.sceneBar';

// initialAuth is a test seam: the smoke test renders the unlocked app without
// running effects (which is where the real auth status arrives).
export function App({ initialAuth = null } = {}) {
  const [config, setConfig] = useState(null);
  const [auth, setAuth] = useState(initialAuth);
  const [sessions, setSessions] = useState([]);
  const [session, setSession] = useState(null);
  const [status, setStatus] = useState(null);
  const [activity, setActivity] = useState([]);
  const [choices, setChoices] = useState([]);
  const [streaming, setStreaming] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [modal, setModal] = useState(null);
  const [tools, setTools] = useState([]);
  const [chatModels, setChatModels] = useState([]);
  const [allTools, setAllTools] = useState([]);
  const [stories, setStories] = useState([]);
  const [editingStory, setEditingStory] = useState(null);
  const [toasts, setToasts] = useState([]);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [panelOpen, setPanelOpen] = useState(false);
  // The scene strip is handy but eats vertical space on a phone, so it can be
  // hidden and the choice sticks across reloads.
  const [sceneBarOpen, setSceneBarOpen] = useState(() => {
    try {
      return typeof window === 'undefined' ? true : localStorage.getItem(LS_SCENE) !== 'hidden';
    } catch (err) {
      return true;
    }
  });
  const [speaking, setSpeaking] = useState(null);

  const abortRef = useRef(null);
  const sessionRef = useRef(null);
  const configRef = useRef(null);
  const speakRef = useRef(null);
  const speechRef = useRef({ current: null, queue: [], busy: false });
  const toastId = useRef(0);
  const bootedRef = useRef(false);
  const streamTokenRef = useRef(0);
  const needsResyncRef = useRef(false);

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  useEffect(() => {
    try {
      if (typeof window !== 'undefined') {
        localStorage.setItem(LS_SCENE, sceneBarOpen ? 'shown' : 'hidden');
      }
    } catch (err) {
      /* a blocked storage must not break the app */
    }
  }, [sceneBarOpen]);

  // Choices, tool activity and usage belong to one session. When the active
  // session goes away (e.g. the last one was deleted) they must not linger.
  useEffect(() => {
    if (!session) {
      setChoices([]);
      setActivity([]);
      setStatus(null);
    }
  }, [session]);

  useEffect(() => {
    configRef.current = config;
  }, [config]);

  // Any 401 from a normal API call means the login session is gone: drop back
  // to the gate and let the next successful login reload everything.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      bootedRef.current = false;
      setSession(null);
      setAuth({ enabled: true, authenticated: false });
    });
  }, []);

  useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const status = await api.authStatus();
        if (alive) setAuth(status);
      } catch (err) {
        // Keep the app usable when status is unreachable; a real 401 will send
        // us back to the login screen anyway.
        if (alive) setAuth({ enabled: false, authenticated: true });
      }
    })();
    return () => { alive = false; };
  }, []);

  const pushToast = useCallback((message, type = 'info') => {
    const id = (toastId.current += 1);
    setToasts((list) => [...list, { id, message, type }]);
    setTimeout(() => setToasts((list) => list.filter((t) => t.id !== id)), type === 'error' ? 6500 : 3800);
  }, []);

  const handleLogin = useCallback(() => {
    bootedRef.current = false;
    setAuth({ enabled: true, authenticated: true });
  }, []);

  const logout = useCallback(async () => {
    try {
      await api.logout();
    } catch (err) { /* the cookie is gone locally either way */ }
    bootedRef.current = false;
    setSession(null);
    setSessions([]);
    setActivity([]);
    setChoices([]);
    setAuth({ enabled: true, authenticated: false });
  }, []);

  // Cancels any in-flight generation and invalidates its late events, so a
  // turn started on one session can never land in another after switching.
  const cancelStream = useCallback(() => {
    streamTokenRef.current += 1;
    if (abortRef.current) abortRef.current.abort();
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
    cancelStream();
    try {
      const data = await api.session(id);
      setSession(data);
      setActivity([]);
      setChoices(data.pendingChoices || []);
      setStatus(null);
      localStorage.setItem(LS_KEY, id);
      setSidebarOpen(false);
      setPanelOpen(false);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [cancelStream, pushToast]);

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
        if (event.message && event.message.role === 'assistant' && event.message.text
            && configRef.current && configRef.current.autoSpeak && speakRef.current) {
          speakRef.current(event.message);
        }
        break;
      case 'state':
        setSession((s) => (s ? { ...s, state: event.state } : s));
        break;
      case 'scene':
        setSession((s) => (s ? { ...s, scene: event.scene } : s));
        break;
      case 'choices':
        setChoices(event.choices || []);
        break;
      case 'reasoning':
        setActivity((list) => [...list, { id: 'reason-' + list.length + '-' + Date.now(), name: 'reasoning', done: true, result: event.text }]);
        break;
      case 'tool_start':
        setActivity((list) => [...list, {
          id: event.tool.id || (event.tool.name + '-' + Date.now()),
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
        // The server folds usage into the session, so the token status stays
        // correct after a reload without another request.
        if (event.tokens) setSession((s) => (s ? { ...s, tokens: event.tokens } : s));
        break;
      case 'notice':
        pushToast(event.text);
        break;
      case 'error':
        pushToast(event.error, 'error');
        // The turn may have been rolled back server-side; resync once the
        // stream ends so the on-screen story matches what was saved.
        needsResyncRef.current = true;
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
    // Events are only applied while this exact run is still the current one.
    const token = (streamTokenRef.current += 1);
    const emit = (event) => {
      if (streamTokenRef.current === token) handleEvent(event);
    };
    setStreaming(true);
    try {
      await api.stream(path, body, emit, controller.signal);
    } catch (err) {
      if (err.name !== 'AbortError') {
        pushToast(err.message, 'error');
        // The turn may have been rolled back server-side; resync so the story
        // on screen matches what was actually saved.
        const current = sessionRef.current;
        if (current) {
          try {
            const fresh = await api.session(current.id);
            setSession(fresh);
            setChoices(fresh.pendingChoices || []);
          } catch (_) { /* keep the optimistic view */ }
        }
      }
    } finally {
      abortRef.current = null;
      setStreaming(false);
      setStatus(null);
      if (needsResyncRef.current) {
        needsResyncRef.current = false;
        const current = sessionRef.current;
        if (current) {
          try {
            const fresh = await api.session(current.id);
            setSession(fresh);
            setChoices(fresh.pendingChoices || []);
          } catch (_) { /* keep what we have */ }
        }
      }
      await refreshSessions();
    }
  }, [handleEvent, pushToast, refreshSessions]);

  const send = useCallback(async (text, files, ooc) => {
    const current = sessionRef.current;
    const value = String(text || '').trim();
    const directive = String(ooc || '').trim();
    const picked = files || [];
    if (!current || (!value && !picked.length && !directive)) return;
    const pending = {
      id: 'pending-' + Date.now(),
      role: 'user',
      kind: 'user',
      speaker: (current.persona && current.persona.name) || '',
      text: value,
      ooc: directive,
      localImages: picked.map((file) => URL.createObjectURL(file)),
      pending: true,
      createdAt: new Date().toISOString(),
    };
    setSession((s) => (s ? { ...s, messages: [...s.messages, pending] } : s));
    setChoices([]);
    try {
      const names = [];
      if (picked.length) {
        setUploading(true);
        for (const file of picked) {
          const up = await uploadAsset(current.id, file);
          names.push(up.name);
        }
      }
      await runStream('/api/sessions/' + current.id + '/messages', { text: value, images: names, ooc: directive });
    } catch (err) {
      pushToast(err.message, 'error');
    } finally {
      setUploading(false);
    }
  }, [runStream, pushToast]);

  // Re-roll one turn: drop everything after its user message and regenerate.
  const rerollFrom = useCallback(async (messageId) => {
    const current = sessionRef.current;
    if (!current || !messageId) return;
    if (!window.confirm('Re-roll from this turn? Every reply after it will be replaced.')) return;
    setSession((s) => {
      if (!s) return s;
      const idx = s.messages.findIndex((m) => m.id === messageId);
      return idx >= 0 ? { ...s, messages: s.messages.slice(0, idx + 1) } : s;
    });
    setChoices([]);
    setActivity([]);
    await runStream('/api/sessions/' + current.id + '/regenerate', { messageId });
  }, [runStream]);

  // Edit a user message and rerun from that point.
  const editUserMessage = useCallback(async (message, text) => {
    const current = sessionRef.current;
    if (!current || !message) return;
    const value = String(text || '').trim();
    if (!value) return;
    setSession((s) => {
      if (!s) return s;
      const idx = s.messages.findIndex((m) => m.id === message.id);
      if (idx < 0) return s;
      const next = s.messages.slice(0, idx + 1);
      next[idx] = { ...next[idx], text: value };
      return { ...s, messages: next };
    });
    setChoices([]);
    setActivity([]);
    await runStream('/api/sessions/' + current.id + '/regenerate', { messageId: message.id, text: value });
  }, [runStream]);

  const stop = useCallback(() => {
    if (abortRef.current) abortRef.current.abort();
  }, []);

  const stopSpeech = useCallback(() => {
    const st = speechRef.current;
    st.queue.length = 0;
    if (st.current) {
      try {
        st.current.audio.pause();
      } catch (err) {
        /* already stopped */
      }
      URL.revokeObjectURL(st.current.url);
      st.current = null;
    }
    st.busy = false;
    setSpeaking(null);
  }, []);

  const runSpeechQueue = useCallback(async () => {
    const st = speechRef.current;
    if (st.busy) return;
    st.busy = true;
    while (st.queue.length) {
      const item = st.queue.shift();
      try {
        setSpeaking(item.id);
        const res = await fetch('/api/sessions/' + item.sessionId + '/speech', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ text: item.text, speaker: item.speaker }),
        });
        if (!res.ok) {
          let message = res.status + ' ' + res.statusText;
          try {
            const body = await res.json();
            if (body && body.error) message = body.error;
          } catch (err) {
            /* not json */
          }
          throw new Error(message);
        }
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const audio = new Audio(url);
        st.current = { id: item.id, audio, url };
        await new Promise((resolve) => {
          audio.onended = resolve;
          audio.onerror = resolve;
          audio.play().catch(resolve);
        });
        try {
          audio.pause();
        } catch (err) {
          /* ignore */
        }
        URL.revokeObjectURL(url);
        st.current = null;
        setSpeaking(null);
      } catch (err) {
        pushToast(err.message, 'error');
        st.current = null;
        setSpeaking(null);
      }
    }
    st.busy = false;
  }, [pushToast]);

  const speak = useCallback((message) => {
    const current = sessionRef.current;
    if (!current || !message || !message.text || !message.id) return;
    const st = speechRef.current;
    if (st.current && st.current.id === message.id) {
      stopSpeech();
      return;
    }
    if (st.queue.some((i) => i.id === message.id)) return;
    st.queue.push({ id: message.id, sessionId: current.id, text: message.text, speaker: message.speaker || '' });
    runSpeechQueue();
  }, [runSpeechQueue, stopSpeech]);

  useEffect(() => {
    speakRef.current = speak;
  }, [speak]);

  const saveConfig = useCallback(async (body) => {
    try {
      const saved = await api.saveConfig(body);
      setConfig(saved);
      // Sessions read the model and generation options from the config on every
      // turn, so nothing has to be copied onto the open story.
      pushToast('Settings saved.');
      setModal(null);
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast]);

  // notify=false keeps the boot-time auto-load quiet when no key is set yet.
  const reloadChatModels = useCallback(async (notify = true) => {
    try {
      const data = await api.models();
      const list = data.models || [];
      setChatModels(list);
      return list;
    } catch (err) {
      if (notify) pushToast(err.message, 'error');
      return [];
    }
  }, [pushToast]);

  const loadStories = useCallback(async () => {
    try {
      const data = await api.stories();
      const list = data.stories || [];
      setStories(list);
      return list;
    } catch (err) {
      return [];
    }
  }, []);

  const openPreset = useCallback(async (id) => {
    if (!id) {
      setEditingStory(null);
      setModal('preset');
      return;
    }
    try {
      setEditingStory(await api.story(id));
      setModal('preset');
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast]);

  const startSession = useCallback(async (data) => {
    cancelStream();
    try {
      const created = await api.createSession(data);
      setModal(null);
      setSession(created);
      setActivity([]);
      setChoices([]);
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
  }, [cancelStream, pushToast, refreshSessions, runStream]);

  const savePreset = useCallback(async (data) => {
    try {
      const picked = (data.characters || []).filter((c) => (c.name || '').trim());
      const payload = {
        title: data.title,
        description: data.description,
        genre: data.genre,
        opening: data.opening,
        persona: data.persona,
        characters: picked.map((c) => ({
          id: c.id, name: c.name, description: c.description,
          personality: c.personality, voice: c.voice || '',
        })),
        settings: data.settings,
      };
      let story = data.id ? await api.patchStory(data.id, payload) : await api.createStory(payload);

      let changed = false;
      let avatar = data.avatar || '';
      if (data.avatarFile) {
        const up = await api.uploadStoryAsset(story.id, data.avatarFile);
        avatar = up.name;
        changed = true;
      }
      const created = story.characters || [];
      const withAvatars = [];
      for (let i = 0; i < picked.length; i++) {
        const src = picked[i];
        const base = created[i] || {};
        let cAvatar = src.avatar || '';
        if (src.avatarFile) {
          const up = await api.uploadStoryAsset(story.id, src.avatarFile);
          cAvatar = up.name;
          changed = true;
        }
        withAvatars.push({
          id: base.id || src.id, name: src.name, description: src.description,
          personality: src.personality, avatar: cAvatar, voice: src.voice || '',
        });
      }
      if (changed) {
        story = await api.patchStory(story.id, { avatar, characters: withAvatars });
      }
      setEditingStory(null);
      setModal('stories');
      await loadStories();
      pushToast('Preset saved.');
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [loadStories, pushToast]);

  const deletePreset = useCallback(async (story) => {
    if (!story) return;
    if (!window.confirm('Delete preset "' + (story.title || '') + '"? Sessions already started from it are kept.')) return;
    try {
      await api.deleteStory(story.id);
      await loadStories();
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [loadStories, pushToast]);

  const loadSpeechModels = useCallback(async () => {
    try {
      const data = await api.speechModels();
      return data.models || [];
    } catch (err) {
      return [];
    }
  }, []);

  const saveStory = useCallback(async (data) => {
    const current = sessionRef.current;
    if (!current) return;
    try {
      const characters = [];
      for (const c of data.characters || []) {
        const name = (c.name || '').trim();
        if (!name) continue;
        let avatar = c.avatar || '';
        if (c.avatarFile) {
          const up = await uploadAsset(current.id, c.avatarFile);
          avatar = up.name;
        }
        characters.push({ id: c.id, name, description: c.description, personality: c.personality, avatar, voice: c.voice || '' });
      }
      let avatar = data.avatar || '';
      if (data.avatarFile) {
        const up = await uploadAsset(current.id, data.avatarFile);
        avatar = up.name;
      }
      const updated = await api.patchSession(current.id, {
        title: data.title,
        avatar,
        characters,
        settings: data.settings,
      });
      setSession(updated);
      setModal(null);
      await refreshSessions();
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [pushToast, refreshSessions]);

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

  const deleteSession = useCallback(async (target) => {
    if (!target) return;
    if (!window.confirm('Delete session "' + (target.title || '') + '"? This cannot be undone.')) return;
    try {
      await api.deleteSession(target.id);
      const list = (await api.sessions()).sessions || [];
      setSessions(list);
      const wasActive = sessionRef.current && sessionRef.current.id === target.id;
      if (wasActive) {
        cancelStream();
        setSession(null);
        localStorage.removeItem(LS_KEY);
        if (list[0]) await openSession(list[0].id);
      }
    } catch (err) {
      pushToast(err.message, 'error');
    }
  }, [cancelStream, openSession, pushToast]);

  const deleteCurrent = useCallback(async () => {
    const current = sessionRef.current;
    if (!current) return;
    if (!window.confirm('Delete this story and all of its images? This cannot be undone.')) return;
    cancelStream();
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
  }, [cancelStream, openSession, pushToast]);

  useEffect(() => {
    if (!auth) return;
    if (auth.enabled && !auth.authenticated) return;
    if (bootedRef.current) return;
    bootedRef.current = true;
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
      await reloadChatModels(false);
      try {
        const toolList = await api.tools();
        setAllTools(toolList.tools || []);
      } catch (err) {
        setAllTools([]);
      }
      await loadStories();
      const last = localStorage.getItem(LS_KEY);
      const target = last && list.some((s) => s.id === last) ? last : list[0] && list[0].id;
      if (target) await openSession(target);
      if (cfg && !cfg.hasKey) pushToast('Add your OpenRouter API key in Settings to begin.');
    })();
  }, [auth]);

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
  if (!auth) {
    return html`
      <div class="login-boot"><span class="dots"><i></i><i></i><i></i></span></div>`;
  }
  if (auth.enabled && !auth.authenticated) {
    return html`
      <${Fragment}>
        <${Login} onSuccess=${handleLogin} />
        <${C.Toasts} toasts=${toasts} />
      <//>`;
  }

  const messages = (session && session.messages) || [];

  return html`
    <${Fragment}>
      <${C.Sidebar}
        sessions=${sessions}
        activeId=${session && session.id}
        open=${sidebarOpen}
        auth=${auth}
        onOpen=${openSession}
        onNew=${() => setModal('new-session')}
        onStories=${() => setModal('stories')}
        onSettings=${openSettings}
        onTools=${openTools}
        onDelete=${deleteSession}
        onLogout=${logout}
      />
      ${(sidebarOpen || panelOpen) && html`<div class="scrim" onClick=${closeDrawers}></div>`}
      <main class="main">
        <${C.Topbar}
          session=${session}
          models=${chatModels}
          model=${config && config.model}
          onMenu=${() => setSidebarOpen((v) => !v)}
          onPanel=${() => setPanelOpen((v) => !v)}
        />
        <${C.SceneBar} scene=${session && session.scene} open=${sceneBarOpen}
                        onToggle=${() => setSceneBarOpen((v) => !v)} />
        <${C.MessageList} session=${session} streaming=${streaming} onNewStory=${() => setModal('new-session')}
                              onSpeak=${speak} speakingId=${speaking}
                              onReroll=${rerollFrom} onEdit=${editUserMessage} busy=${streaming} />
        <${C.StatusBar} status=${status && status.status} step=${status && status.step} />
        <${C.Choices} choices=${session ? choices : []} onChoose=${ (text) => send(text, []) } />
        <${C.Composer}
          streaming=${streaming}
          uploading=${uploading}
          disabled=${!session}
          hasChoices=${choices.length > 0}
          onSend=${send}
          onStop=${stop}
        />
      </main>
      <${C.Panel}
        session=${session}
        activity=${activity}
        open=${panelOpen}
        models=${chatModels}
        model=${config && config.model}
        sceneBar=${sceneBarOpen}
        onToggleSceneBar=${() => setSceneBarOpen((v) => !v)}
        onClose=${() => setPanelOpen(false)}
        onEditCast=${() => setModal('story')}
      />
      ${modal === 'settings' && html`
        <${C.SettingsModal} config=${config} onClose=${() => setModal(null)} onSave=${saveConfig}
                            chatModels=${chatModels} onReloadModels=${reloadChatModels}
                            onLoadSpeechModels=${loadSpeechModels} tools=${allTools} />`}
      ${modal === 'new-session' && html`
        <${NewSessionModal} stories=${stories} onClose=${() => setModal(null)} onCreate=${startSession} />`}
      ${modal === 'stories' && html`
        <${StoriesModal} stories=${stories} onClose=${() => setModal(null)}
                         onStart=${async (id) => { await loadStories(); await startSession({ storyId: id }); }}
                         onEdit=${openPreset} onNew=${() => openPreset(null)} onDelete=${deletePreset} />`}
      ${modal === 'preset' && html`
        <${PresetModal} story=${editingStory} onClose=${() => setModal(null)} onSave=${savePreset} />`}
      ${modal === 'story' && session && html`
        <${StoryModal} session=${session} onClose=${() => setModal(null)} onSave=${saveStory}
                       speechModel=${config && config.speechModel}
                       onLoadSpeechModels=${loadSpeechModels} />`}
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
