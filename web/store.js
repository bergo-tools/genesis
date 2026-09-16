// Small shared client state. Rendering lives in ui.js, effects in app.js.
export const state = {
  config: null,
  sessions: [],
  session: null,
  tools: [],
  streaming: false,
  choices: [],
  choicesPrompt: '',
  activity: [],
  usage: null,
  abort: null,
};

export function activeCharacter() {
  const chars = state.session && state.session.characters;
  return chars && chars.length ? chars[0] : null;
}

export function findCharacter(speaker) {
  const chars = state.session && state.session.characters;
  if (!chars || !speaker) return null;
  const needle = String(speaker).toLowerCase();
  return chars.find((c) => (c.name || '').toLowerCase() === needle) || null;
}
