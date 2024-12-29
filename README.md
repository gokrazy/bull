# gokrazy/bull 🐮

* bull is a minimalist bullet journaling program
* less is more
* bull is 1/3rd less than bullet. bull~~et~~ — get it?

## installation

If you want to use bull without modifying its source, [install Go 1.21 or
newer](https://go.dev/dl) and run:

    go install github.com/gokrazy/bull/cmd/bull@latest

(bull needs Go 1.24rc1 or newer for its [os.Root
type](https://pkg.go.dev/os@go1.24rc1#Root), but Go 1.21 [introduced forward
compatibility](https://go.dev/blog/toolchain) in the form of toolchain
management.)

**Tip:** When deploying bull to other systems, I recommend building with the
`CGO_ENABLED=0` environment variable, which will result in a statically linked
binary program without dependencies on the system libc. That makes it easy to,
for example, build on Arch Linux and deploy on Debian stable.

### installation for development

To make changes to the bull code, clone the git repository:

    git clone https://github.com/gokrazy/bull

Then, change into the newly created bull directory and run:

    go install ./cmd/bull

## details

bull uses the yuin/goldmark markdown renderer, specifically:
* with the wikilink extension: https://github.com/abhinav/goldmark-wikilink

* renders backlinks at the end of a page
  * we probably do not want a graph

* live reload: when a page changes, the browser reloads

* opt-in editor: CodeMirror (or textarea when built with nocodemirror)

* special pages:
  * /_bull/mostrecent

## terminology

* content directory (-content flag)
* content page name: relative to -content directory, no .md
* content file name: relative to -content directory, with .md

## out of scope

* rendering non-local input. if you need to display remote content, mirror it to
  a local (ram)disk before starting bull.
* rendering non-markdown. it sure seems tempting to add just one more file
  format, but if you want bull to display it, convert it to markdown.

## planned improvements

* inotify / fswatch instead of polling for live reload
* customization: make it easy to build your own bull
  * you can leave a favicon.ico in your content directory
* handle front matter better (ignore? format differently?), e.g. [[untagged/prober7]]
* generate IDs for each heading
* make each heading foldable
* backlinks: do not list the same page multiple times ([[SETTINGS]])
* codemirror: enable line breaks (instead of scrolling)
* edit: move save button to top
* mostrecent: use parallel walk
* mostrecent: paginate to make the page manageable for large gardens
* content settings: make title_format configurable

## why not just…?

* use hugo? it already renders markdown files and implements live reload
  * hugo has *opinions* about file structure
    * e.g. regarding case sensitivity, 
	* or an index.md in a directory making all other .md files inaccessible
	* URLs end in / automatically
  * hugo’s code base is pretty large (`wc -l` reports 190'000 lines)
  * the point is to have a minimalist, understandable markdown viewer without
    opinions about file names
