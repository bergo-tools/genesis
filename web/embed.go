// Package web embeds the single-page frontend into the binary so Genesis ships
// as one self-contained executable.
package web

import "embed"

// Files holds the browser client. Preact and htm are vendored as plain ESM
// builds, so there is no npm install and no bundler step.
//
//go:embed index.html app.css app.js api.js format.js components.js story.js library.js vendor/preact.module.js vendor/hooks.module.js vendor/htm.module.js
var Files embed.FS
