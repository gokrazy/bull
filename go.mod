module github.com/gokrazy/bull

go 1.24rc2

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c
	github.com/fsnotify/fsnotify v1.8.0
	github.com/yuin/goldmark v1.7.8
	go.abhg.dev/goldmark/wikilink v0.5.0
	golang.org/x/image v0.23.0
	golang.org/x/sync v0.10.0
	golang.org/x/tools v0.28.0
	mvdan.cc/xurls/v2 v2.6.0
)

require (
	github.com/evanw/esbuild v0.24.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	honnef.co/go/tools v0.5.1 // indirect
)

tool github.com/evanw/esbuild/cmd/esbuild

tool honnef.co/go/tools/cmd/staticcheck
