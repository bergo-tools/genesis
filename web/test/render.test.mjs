// Headless smoke test: build the whole Preact tree and render it to a string.
// Effects do not run, so this validates the components and htm templates
// without a browser or any npm install.
import renderToString from './render-to-string.module.js';
import { h } from '../vendor/preact.module.js';
import { App } from '../app.js';

const out = renderToString(h(App, {}));
const checks = ['Genesis', 'Begin a story', 'What do you do?', 'agentic roleplay'];
const missing = checks.filter((needle) => !out.includes(needle));
if (missing.length) {
  console.error('SSR render is missing: ' + missing.join(', '));
  process.exit(1);
}
console.log('web SSR smoke test OK (' + out.length + ' chars)');
