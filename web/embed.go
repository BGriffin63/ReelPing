// Package webassets embeds ReelPing's HTML templates and static files so the
// binary is fully self-contained (no external files, CDNs, or fonts).
package webassets

import "embed"

// Templates holds the HTML templates.
//
//go:embed templates/*.html
var Templates embed.FS

// Static holds CSS, JS, and icon assets served under /static.
//
//go:embed static/*
var Static embed.FS
