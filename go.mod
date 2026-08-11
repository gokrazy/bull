module github.com/gokrazy/bull

go 1.26

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/fsnotify/fsnotify v1.10.1
	github.com/google/go-cmp v0.7.0
	github.com/google/renameio/v2 v2.0.2
	github.com/yuin/goldmark v1.8.2
	go.abhg.dev/goldmark/wikilink v0.6.0
	golang.org/x/image v0.42.0
	golang.org/x/sync v0.21.0
	golang.org/x/tools v0.47.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/evanw/esbuild v0.28.1 // indirect
	github.com/go-json-experiment/json v0.0.0-20260623181947-01eb4420fa68 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/mackerelio/go-osstat v0.2.7 // indirect
	github.com/microsoft/typescript-go v0.0.0-20260708042240-2bd066d87f5b // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.37.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/term v0.44.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	honnef.co/go/tools v0.7.0 // indirect
)

tool (
	github.com/evanw/esbuild/cmd/esbuild
	github.com/microsoft/typescript-go/cmd/tsgo
	honnef.co/go/tools/cmd/staticcheck
)
