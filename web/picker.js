import { h } from "./vendor/preact.module.js";
import { useEffect, useRef, useState } from "./vendor/hooks.module.js";
import htm from "./vendor/htm.module.js";

const html = htm.bind(h);

// A model list can hold hundreds of entries and native <datalist> is either
// invisible (iOS Safari) or cramped (Android Chrome), so every model/voice
// choice goes through this searchable sheet instead.
const MAX_ITEMS = 400;

function norm(text) {
  return String(text == null ? "" : text).toLowerCase();
}

export function modelOptions(models) {
  return (models || []).map((m) => ({
    value: m.id,
    label: m.name || m.id,
    hint: [
      m.id,
      m.tools ? "tools" : "",
      m.context ? Math.round(m.context / 1000) + "k ctx" : "",
      m.maxOutput ? Math.round(m.maxOutput / 1000) + "k out" : "",
    ].filter(Boolean).join(" · "),
  }));
}

export function speechOptions(models) {
  return (models || []).map((m) => ({
    value: m.id,
    label: m.name || m.id,
    hint: m.id + " · " + (m.voices || []).length + " voices",
  }));
}

export function stringOptions(values) {
  return (values || []).map((v) => ({ value: v, label: v }));
}

export function OptionPicker({ value, options, onChange, placeholder, title, allowCustom, disabled, emptyText }) {
  const opts = options || [];
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const inputRef = useRef(null);
  const custom = allowCustom !== false;
  const typed = query.trim();
  const q = norm(typed);
  const current = opts.find((o) => o.value === value);
  const label = current ? current.label : value;
  const sub = current && current.value !== current.label ? current.value : "";
  let filtered = q ? opts.filter((o) => norm(o.label).includes(q) || norm(o.hint).includes(q)) : opts;
  let extra = 0;
  if (filtered.length > MAX_ITEMS) {
    extra = filtered.length - MAX_ITEMS;
    filtered = filtered.slice(0, MAX_ITEMS);
  }
  const showCustom = custom && typed !== "" && !opts.some((o) => o.value === typed);

  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    const timer = setTimeout(() => {
      if (inputRef.current) inputRef.current.focus();
    }, 40);
    return () => {
      document.removeEventListener("keydown", onKey);
      clearTimeout(timer);
    };
  }, [open]);

  const openPicker = () => {
    if (disabled) return;
    setQuery("");
    setOpen(true);
  };
  const choose = (next) => {
    setOpen(false);
    if (next !== value) onChange(next);
  };

  return html`
    <button type="button" class="picker-trigger" disabled=${!!disabled} onClick=${openPicker} title=${value || ""}>
      ${label
        ? html`<span class="picker-value">${label}${sub ? html`<em class="picker-sub">${sub}</em>` : null}</span>`
        : html`<span class="picker-value is-empty">${placeholder || "Select"}</span>`}
      <span class="picker-caret">▾</span>
    </button>
    ${open && html`
      <div class="picker-overlay" onClick=${ (e) => { if (e.target === e.currentTarget) setOpen(false); } }>
        <div class="picker-sheet">
          <header class="picker-head">
            <strong>${title || "Select"}</strong>
            <button class="icon-btn" type="button" onClick=${ () => setOpen(false) }>×</button>
          </header>
          <div class="picker-search-row">
            <input ref=${inputRef} class="picker-search" type="search" enterkeyhint="search"
                   placeholder="Search or type an id" value=${query}
                   onInput=${ (e) => setQuery(e.currentTarget.value) } />
          </div>
          <div class="picker-list">
            ${showCustom && html`
              <button type="button" class="picker-item custom" onClick=${ () => choose(typed) }>
                <span class="picker-item-label">Use ${typed}</span>
              </button>`}
            ${filtered.map((o) => html`
              <button type="button" key=${o.value} class=${"picker-item" + (o.value === value ? " active" : "")}
                      onClick=${ () => choose(o.value) }>
                <span class="picker-item-label">${o.label}</span>
                ${o.hint ? html`<span class="picker-item-hint">${o.hint}</span>` : null}
              </button>`)}
            ${extra > 0 && html`<p class="hint picker-empty">${extra} more. Keep typing to narrow.</p>`}
            ${filtered.length === 0 && !showCustom && html`<p class="hint picker-empty">${emptyText || "No matches."}</p>`}
          </div>
        </div>
      </div>`}
  `;
}
