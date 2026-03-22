package assets

import "embed"

//go:embed *.html.tmpl js/*.js css/*.css svg/*.svg *.xml.tmpl favicon.ico favicon-32x32.png apple-touch-icon.png
var FS embed.FS
