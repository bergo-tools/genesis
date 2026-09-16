const JSON_HEADERS = { 'Content-Type': 'application/json' };
const AUTH_PREFIX = '/api/auth/';

// onUnauthorized fires when the server rejects a request because the login
// session is gone (expired, or the process restarted). The app uses it to show
// the login screen again.
let onUnauthorized = null;
export function setUnauthorizedHandler(fn) { onUnauthorized = fn; }

function noteUnauthorized(url, res) {
  if (res.status === 401 && onUnauthorized && !url.startsWith(AUTH_PREFIX)) onUnauthorized();
}

async function toJSON(res) {
  if (!res.ok) {
    let message = res.status + ' ' + res.statusText;
    try {
      const body = await res.json();
      if (body && body.error) message = body.error;
    } catch (_) { /* not json */ }
    throw new Error(message);
  }
  if (res.status === 204) return null;
  return res.json();
}

async function requestJSON(url, options) {
  const res = await fetch(url, options);
  noteUnauthorized(url, res);
  return toJSON(res);
}

function post(url, body) {
  return requestJSON(url, { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body || {}) });
}

function patch(url, body) {
  return requestJSON(url, { method: 'PATCH', headers: JSON_HEADERS, body: JSON.stringify(body) });
}

// assetURL builds the browser URL for an uploaded image.
export function assetURL(sessionId, name) {
  if (!sessionId || !name) return '';
  return '/api/sessions/' + encodeURIComponent(sessionId) + '/assets/' + encodeURIComponent(name);
}

// storyAssetURL builds the browser URL for a preset asset.
export function storyAssetURL(storyId, name) {
  if (!storyId || !name) return '';
  return '/api/stories/' + encodeURIComponent(storyId) + '/assets/' + encodeURIComponent(name);
}

async function uploadTo(base, id, file) {
  const form = new FormData();
  form.append('file', file, file.name || 'upload');
  return requestJSON(base + '/' + encodeURIComponent(id) + '/assets', { method: 'POST', body: form });
}

// uploadAsset posts a File to a session and resolves to { name, url }.
export function uploadAsset(sessionId, file) {
  return uploadTo('/api/sessions', sessionId, file);
}

// uploadStoryAsset posts a File to a preset.
export function uploadStoryAsset(storyId, file) {
  return uploadTo('/api/stories', storyId, file);
}

export const api = {
  authStatus: () => requestJSON('/api/auth/status'),
  login: (password) => post('/api/auth/login', { password }),
  logout: () => post('/api/auth/logout'),
  config: () => requestJSON('/api/config'),
  saveConfig: (c) => requestJSON('/api/config', { method: 'PUT', headers: JSON_HEADERS, body: JSON.stringify(c) }),
  models: () => requestJSON('/api/models'),
  tools: () => requestJSON('/api/tools'),
  speechModels: () => requestJSON('/api/speech/models'),
  stories: () => requestJSON('/api/stories'),
  story: (id) => requestJSON('/api/stories/' + encodeURIComponent(id)),
  createStory: (body) => post('/api/stories', body),
  patchStory: (id, body) => patch('/api/stories/' + encodeURIComponent(id), body),
  deleteStory: (id) => requestJSON('/api/stories/' + encodeURIComponent(id), { method: 'DELETE' }),
  uploadStoryAsset,
  sessions: () => requestJSON('/api/sessions'),
  createSession: (body) => post('/api/sessions', body),
  session: (id) => requestJSON('/api/sessions/' + encodeURIComponent(id)),
  patchSession: (id, body) => patch('/api/sessions/' + encodeURIComponent(id), body),
  deleteSession: (id) => requestJSON('/api/sessions/' + encodeURIComponent(id), { method: 'DELETE' }),
  uploadAsset,
  stream: streamNDJSON,
};

// streamNDJSON POSTs a request and invokes onEvent for each newline-delimited
// JSON event the server streams back.
export async function streamNDJSON(path, body, onEvent, signal) {
  const res = await fetch(path, {
    method: 'POST',
    headers: JSON_HEADERS,
    body: JSON.stringify(body || {}),
    signal,
  });
  noteUnauthorized(path, res);
  if (!res.ok || !res.body) {
    let message = res.status + ' ' + res.statusText;
    try {
      const parsed = await res.json();
      if (parsed && parsed.error) message = parsed.error;
    } catch (_) { /* ignore */ }
    throw new Error(message);
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    let index;
    while ((index = buffer.indexOf('\n')) >= 0) {
      const line = buffer.slice(0, index).trim();
      buffer = buffer.slice(index + 1);
      if (!line) continue;
      try {
        onEvent(JSON.parse(line));
      } catch (err) {
        console.warn('genesis: ignoring malformed event', line, err);
      }
    }
  }
  const tail = buffer.trim();
  if (tail) {
    try {
      onEvent(JSON.parse(tail));
    } catch (_) { /* ignore */ }
  }
}
