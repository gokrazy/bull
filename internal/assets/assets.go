package assets

import "embed"

//go:embed *.html.tmpl js/*.js css/*.css svg/*.svg *.xml.tmpl
var FS embed.FS
