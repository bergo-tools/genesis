const JSON_HEADERS = { 'Content-Type': 'application/json' };

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
  const res = await fetch(base + '/' + encodeURIComponent(id) + '/assets', { method: 'POST', body: form });
  return toJSON(res);
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
  config: () => fetch('/api/config').then(toJSON),
  saveConfig: (c) => fetch('/api/config', { method: 'PUT', headers: JSON_HEADERS, body: JSON.stringify(c) }).then(toJSON),
  models: () => fetch('/api/models').then(toJSON),
  tools: () => fetch('/api/tools').then(toJSON),
  speechModels: () => fetch('/api/speech/models').then(toJSON),
  stories: () => fetch('/api/stories').then(toJSON),
  story: (id) => fetch('/api/stories/' + encodeURIComponent(id)).then(toJSON),
  createStory: (body) => fetch('/api/stories', { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body) }).then(toJSON),
  patchStory: (id, body) => fetch('/api/stories/' + encodeURIComponent(id), { method: 'PATCH', headers: JSON_HEADERS, body: JSON.stringify(body) }).then(toJSON),
  deleteStory: (id) => fetch('/api/stories/' + encodeURIComponent(id), { method: 'DELETE' }).then(toJSON),
  uploadStoryAsset,
  sessions: () => fetch('/api/sessions').then(toJSON),
  createSession: (body) => fetch('/api/sessions', { method: 'POST', headers: JSON_HEADERS, body: JSON.stringify(body) }).then(toJSON),
  session: (id) => fetch('/api/sessions/' + encodeURIComponent(id)).then(toJSON),
  patchSession: (id, body) => fetch('/api/sessions/' + encodeURIComponent(id), { method: 'PATCH', headers: JSON_HEADERS, body: JSON.stringify(body) }).then(toJSON),
  deleteSession: (id) => fetch('/api/sessions/' + encodeURIComponent(id), { method: 'DELETE' }).then(toJSON),
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
