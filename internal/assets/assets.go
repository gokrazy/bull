package assets

import "embed"

//go:embed *.html.tmpl js/*.js *.xml.tmpl
var FS embed.FS
