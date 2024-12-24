# gokrazy/bull 🐮

* bull is a minimalist bullet journaling program
* less is more
* bull is 1/3rd less than bullet. bull~~et~~ — get it?

## installation

    go install github.com/gokrazy/bull/cmd/bull@latest

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

## why not just…?

* use hugo? it already renders markdown files and implements live reload
  * hugo has *opinions* about file structure
    * e.g. regarding case sensitivity, 
	* or an index.md in a directory making all other .md files inaccessible
	* URLs end in / automatically
  * hugo’s code base is pretty large (`wc -l` reports 190'000 lines)
  * the point is to have a minimalist, understandable markdown viewer without
    opinions about file names
