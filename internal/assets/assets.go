package assets

import "embed"

//go:embed *.html.tmpl js/*.js
var FS embed.FS
