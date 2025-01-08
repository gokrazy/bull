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
	pb = bytes.ReplaceAll(pb, []byte("[["+oldpg+"]]"), []byte("[["+newpg+"]]"))
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
	bull.idx = idx
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
