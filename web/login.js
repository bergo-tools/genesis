import { h } from './vendor/preact.module.js';
import { useEffect, useRef, useState } from './vendor/hooks.module.js';
import htm from './vendor/htm.module.js';
import { api } from './api.js';

const html = htm.bind(h);

// Login is the full-screen gate shown when GENESIS_PASSWORD is configured and
// the browser has no live session yet.
export function Login({ onSuccess }) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const inputRef = useRef(null);

  useEffect(() => {
    if (inputRef.current) inputRef.current.focus();
  }, []);

  const submit = async (event) => {
    if (event && event.preventDefault) event.preventDefault();
    if (!password.trim() || busy) return;
    setBusy(true);
    setError('');
    try {
      await api.login(password);
      setPassword('');
      if (onSuccess) await onSuccess();
    } catch (err) {
      setError(err && err.message ? err.message : 'Could not sign in.');
      setPassword('');
      if (inputRef.current) inputRef.current.focus();
    } finally {
      setBusy(false);
    }
  };

  return html`
    <div class="login-screen">
      <form class="login-card" onSubmit=${submit}>
        <div class="login-mark">◆</div>
        <h1>Genesis</h1>
        <p class="muted">Enter the password to open your stories.</p>
        <label class="field">
          <span>Password</span>
          <input ref=${inputRef} type="password" value=${password} autocomplete="current-password"
                 placeholder="••••••••" onInput=${(e) => setPassword(e.currentTarget.value)} />
        </label>
        ${error ? html`<p class="login-error">${error}</p>` : null}
        <button class="btn btn-primary btn-block" type="submit" disabled=${busy || !password.trim()}>
          ${busy ? 'Unlocking…' : 'Unlock'}
        </button>
      </form>
    </div>`;
}
