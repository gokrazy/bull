package assets

import "embed"

//go:embed *.html.tmpl js/*.js css/*.css *.xml.tmpl
var FS embed.FS
