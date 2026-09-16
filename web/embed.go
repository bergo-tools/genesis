// Package web embeds the single-page frontend into the binary so Genesis ships
// as one self-contained executable.
package web

import "embed"

//go:embed index.html app.css app.js api.js ui.js store.js
var Files embed.FS
