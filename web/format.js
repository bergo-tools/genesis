// Formatting helpers shared by the Preact components.
export function escapeHTML(value) {
  return String(value == null ? '' : value).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

// formatText returns a safe HTML string: the input is escaped first, then a
// tiny markdown subset (**bold**, *italic*, `code`) is applied. Escaping
// happens before any tag is introduced, so the result is safe to hand to
// dangerouslySetInnerHTML.
export function formatText(text) {
  let out = escapeHTML(text || '');
  out = out.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  out = out.replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>');
  out = out.replace(/`([^`]+)`/g, '<code>$1</code>');
  out = out.replace(/\n/g, '<br>');
  return out;
}

export function initial(name) {
  const value = String(name || '').trim();
  return value ? value.charAt(0).toUpperCase() : '?';
}
