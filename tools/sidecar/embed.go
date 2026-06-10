// Package sidecar embeds the Node transformer-plugin worker so released
// rotor binaries can run transformer plugins without a checkout of this
// repository. node_modules is intentionally not embedded: the worker
// resolves `typescript` from the target project at runtime.
package sidecar

import "embed"

//go:embed main.js index.js lib/*.js package.json
var FS embed.FS
