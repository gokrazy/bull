# gokrazy/bull

<img src="https://raw.githubusercontent.com/gokrazy/bull/refs/heads/main/internal/assets/svg/bull-logo.svg" width="200" align="right" alt="bull logo">

* bull is a minimalist bullet journaling program
* less is more
* bull is 1/3rd less than bullet. bull~~et~~ — get it?

## installation

If you want to use bull without modifying its source, [install Go 1.21 or
newer](https://go.dev/dl) and run:

    go install github.com/gokrazy/bull/cmd/bull@latest

(bull needs Go 1.24 or newer for its [os.Root
type](https://pkg.go.dev/os@go1.24#Root), but Go 1.21 [introduced forward
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

If you want to modify the TypeScript code (for codemirror editor integration),
you also need the TypeScript compiler (`tsc`). You can [use
Nix](https://michael.stapelberg.ch/posts/2025-07-27-dev-shells-with-nix-4-quick-examples/)
for that:

    nix develop
	./regenerate.sh

## key differentiators

* made for external editing (e.g. with Emacs or your favorite editor)
  * combines well with remote editing support ([Emacs
    TRAMP](https://www.gnu.org/software/tramp/)), e.g. `emacs
    /ssh:keep:/srv/keep/days/2026-03-27.md`
  * but, as a low-barrier option, a web editor (CodeMirror) is also available
* no state, all pages are indexed on startup (quickly)
* no cache, every request is server-rendered (no PWA)
* fast search across page names and content
* easy content ingestion via `curl`:
  * e.g. `ls -lR /srv/data/mp3 | curl -F 'markdown=<-' http://keep.lan/_bull/save/inbox/storage2-list`
* command-line tools like `bull graph` and `bull mv` help analyze / restructure your knowledge garden
* one or many: bull is relocatable! e.g. I can host `--root=/michael/` and `--root=/wife/` on the family server

## details

bull uses the yuin/goldmark markdown renderer, specifically:
* with the wikilink extension: https://github.com/abhinav/goldmark-wikilink

* renders backlinks at the end of a page
  * we probably do not want a visual graph visualization (too fancy)

* live reload: when a page changes, the browser reloads

* opt-in editor: CodeMirror (or textarea when built with nocodemirror)

* special pages:
  * /_bull/mostrecent or /_bull/browse directory browser in general

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

* more comprehensive customization support: make it easy to integrate your own bull
  * you can leave a favicon.ico in your content directory
* make each heading foldable
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
