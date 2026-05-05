package bull

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

const mvUsage = `
mv - rename markdown page and update links

Syntax:
  % bull mv <src> <dest>

src and dest can be either file names (ending in .md)
or page names (without an .md suffix).

Examples:
  % bull mv simd Performance/SIMD
  % bull mv simd.md Performance/SIMD.md
`

func (b *bullServer) replaceLinks(linker, oldpg, newpg string) error {
	f, err := b.content.Open(linker)
	if err != nil {
		return err
	}
	pb, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		return err
	}
	pb = replaceWikilinkTargets(pb, oldpg, newpg)
	f, err = b.content.OpenFile(linker, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(pb); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

// replaceWikilinkTargets rewrites every [[oldpg…]] / ![[oldpg…]] in src to use
// newpg as the target, preserving any fragment, label, and the GFM table-cell
// pipe escape ('\|'). Forms handled:
//
//	[[oldpg]]
//	[[oldpg|label]]
//	[[oldpg\|label]]   (table-escaped form)
//	[[oldpg#frag]]
//	[[oldpg#frag|label]]
//	![[oldpg…]]        (embed)
func replaceWikilinkTargets(src []byte, oldpg, newpg string) []byte {
	var out bytes.Buffer
	out.Grow(len(src))
	for i := 0; i < len(src); {
		var prefix int
		switch {
		case i+1 < len(src) && src[i] == '[' && src[i+1] == '[':
			prefix = 2
		case i+2 < len(src) && src[i] == '!' && src[i+1] == '[' && src[i+2] == '[':
			prefix = 3
		default:
			out.WriteByte(src[i])
			i++
			continue
		}
		contentStart := i + prefix
		closeRel := bytes.Index(src[contentStart:], []byte("]]"))
		if closeRel < 0 {
			out.WriteByte(src[i])
			i++
			continue
		}
		contentEnd := contentStart + closeRel
		inner := src[contentStart:contentEnd]
		target, consumed := parseWikilinkTarget(inner)
		if string(target) == oldpg {
			out.Write(src[i:contentStart])
			out.WriteString(newpg)
			out.Write(inner[consumed:])
			out.WriteString("]]")
		} else {
			out.Write(src[i : contentEnd+2])
		}
		i = contentEnd + 2
	}
	return out.Bytes()
}

// parseWikilinkTarget returns the page-name portion of a wikilink's inner
// content (the bytes between "[[" and "]]") and the number of bytes that
// belong to it in the source. It mirrors the wikilink parser: split on the
// first '|' (the label separator), then on the last '#' (the fragment).
// A trailing '\' on the page name (the GFM table-cell pipe escape) is
// stripped, but only when a '|' was actually consumed — otherwise a target
// that legitimately ends in '\' would be silently truncated.
func parseWikilinkTarget(inner []byte) (target []byte, consumed int) {
	beforePipe := inner
	if pipe := bytes.IndexByte(inner, '|'); pipe >= 0 {
		beforePipe = inner[:pipe]
		if len(beforePipe) > 1 && beforePipe[len(beforePipe)-1] == '\\' {
			beforePipe = beforePipe[:len(beforePipe)-1]
		}
	}
	if hash := bytes.LastIndexByte(beforePipe, '#'); hash >= 0 {
		return beforePipe[:hash], hash
	}
	return beforePipe, len(beforePipe)
}

func mv(args []string) error {
	fset := flag.NewFlagSet("mv", flag.ExitOnError)
	fset.Usage = usage(fset, mvUsage)

	var dryRun = fset.Bool("dry_run", false, "do not actually move the page, only print actions")

	if err := fset.Parse(args); err != nil {
		return err
	}

	if fset.NArg() != 2 {
		return fmt.Errorf("syntax: mv <src> <dest>")
	}

	dryRunPrefix := func() string {
		if *dryRun {
			return "[dry-run] "
		}
		return ""
	}

	content, err := os.OpenRoot(*contentDir)
	if err != nil {
		return err
	}

	cs, err := loadContentSettings(content)
	if err != nil {
		return err
	}

	bull := &bullServer{
		content:         content,
		contentDir:      *contentDir,
		contentSettings: cs,
		contentChanged:  make(chan struct{}),
	}
	if err := bull.init(); err != nil {
		return err
	}

	start := time.Now()
	log.Printf("indexing all pages (markdown files) in %s (for backlinks)", content.Name())
	idx, err := bull.index()
	if err != nil {
		return err
	}
	bull.idx.Store(idx)
	log.Printf("discovered in %.2fs: directories: %d, pages: %d, links: %d", time.Since(start).Seconds(), idx.dirs, idx.pages, len(idx.backlinks))

	src := fset.Arg(0)
	possibilities := page2files(src)
	if isMarkdown(src) {
		possibilities = []string{src}
	}
	pg, err := bull.readFirst(possibilities)
	if err != nil {
		return err
	}
	dest := fset.Arg(1)
	destPage := file2page(dest)
	if !isMarkdown(dest) {
		destPage = dest
		dest = page2desired(dest)
	}

	log.Printf("%smv %q %q", dryRunPrefix(), pg.FileName, dest)
	if !*dryRun {
		oldpath := filepath.Join(bull.contentDir, pg.FileName)
		newpath := filepath.Join(bull.contentDir, dest)
		if err := mkdirAll(bull.content, filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.Rename(oldpath, newpath); err != nil {
			return err
		}
	}
	log.Printf("# backlinks: %d", len(idx.backlinks[pg.PageName]))
	for _, linker := range idx.backlinks[pg.PageName] {
		linkerpg, err := bull.readFirst(page2files(linker))
		if err != nil {
			log.Printf("  not found: %v", err)
			continue
		}

		log.Printf(`%sreplace [[%s]] → [[%s]] in %s`, dryRunPrefix(), pg.PageName, destPage, linkerpg.FileName)

		if *dryRun {
			continue
		}

		if err := bull.replaceLinks(linkerpg.FileName, pg.PageName, destPage); err != nil {
			log.Printf("  failed: %v", err)
		}
	}

	return nil
}
