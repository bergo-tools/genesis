import { h } from './vendor/preact.module.js';
import htm from './vendor/htm.module.js';
import { formatTokens } from './format.js';

const html = htm.bind(h);

// contextFor finds the model's context window, when the catalog is loaded.
function contextFor(session, models) {
  const model = (models || []).find((m) => m.id === (session && session.model));
  return (model && model.context) || 0;
}

// TokenStatus is the compact context gauge shown in the top bar.
export function TokenStatus({ session, models }) {
  const tokens = (session && session.tokens) || {};
  const limit = contextFor(session, models);
  const used = tokens.lastPromptTokens || 0;
  const pct = limit && used ? Math.min(100, Math.round((used / limit) * 100)) : 0;
  const level = pct >= 90 ? ' danger' : pct >= 70 ? ' warn' : '';
  const label = used ? formatTokens(used) + (limit ? '/' + formatTokens(limit) : '') : '—';
  const title = 'Context ' + (limit ? (pct + '% of ' + formatTokens(limit)) : 'window unknown') +
    ' · prompt ' + used + ' · cached ' + (tokens.lastCachedTokens || 0) +
    ' · session ' + formatTokens((tokens.totalPromptTokens || 0) + (tokens.totalCompletionTokens || 0)) + ' tokens';
  return html`
    <span class=${'token-status' + level} title=${title}>
      <span class="token-bar"><i style=${'width:' + pct + '%'}></i></span>
      <span class="token-num">${label}</span>
    </span>`;
}

// TokenPanel is the detailed breakdown inside the world panel.
export function TokenPanel({ session, models }) {
  const tokens = (session && session.tokens) || {};
  const limit = contextFor(session, models);
  const used = tokens.lastPromptTokens || 0;
  const total = (tokens.totalPromptTokens || 0) + (tokens.totalCompletionTokens || 0);
  const context = used
    ? formatTokens(used) + (limit ? ' / ' + formatTokens(limit) + ' (' + Math.round((used / limit) * 100) + '%)' : '')
    : '—';
  const rows = [
    ['Context', context],
    ['Cached', formatTokens(tokens.lastCachedTokens || 0)],
    ['Prompt total', formatTokens(tokens.totalPromptTokens || 0)],
    ['Completion total', formatTokens(tokens.totalCompletionTokens || 0)],
    ['Session total', formatTokens(total)],
    ['Requests', String(tokens.requests || 0)],
  ];
  if (tokens.totalCost) rows.push(['Cost', '$' + Number(tokens.totalCost).toFixed(4)]);
  return html`
    <section class="panel-section">
      <h3>Tokens</h3>
      <div class="stat-grid">
        ${rows.map(([label, value]) => html`<div class="stat" key=${label}><span>${label}</span><b>${value}</b></div>`)}
      </div>
    </section>`;
}
