package bull

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"
)

const graphUsage = `
graph - show link graph, orphans, and broken links

Syntax:
  % bull graph [--output=text|json]

Examples:
  % bull --content ~/keep graph
  % bull --content ~/keep graph --output=json
  % bull --content ~/keep graph --output=json | jq .stats
`

type graphOutput struct {
	Pages       map[string]graphPage `json:"pages"`
	Orphans     []string             `json:"orphans"`
	BrokenLinks []brokenLink         `json:"broken_links"`
	Stats       graphStats           `json:"stats"`
}

type graphPage struct {
	Outgoing []string `json:"outgoing"`
	Incoming []string `json:"incoming"`
}

type brokenLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type graphStats struct {
	TotalPages      int `json:"total_pages"`
	TotalLinks      int `json:"total_links"`
	OrphanCount     int `json:"orphan_count"`
	BrokenLinkCount int `json:"broken_link_count"`
}

func graph(args []string) error {
	fset := flag.NewFlagSet("graph", flag.ExitOnError)
	fset.Usage = usage(fset, graphUsage)
	output := fset.String("output", "text", "output format: text or json")

	if err := fset.Parse(args); err != nil {
		return err
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
	idx, err := bull.index()
	if err != nil {
		return err
	}
	elapsed := time.Since(start)

	// Compute total links
	totalLinks := 0
	for _, targets := range idx.links {
		totalLinks += len(targets)
	}

	// Find orphan pages (no incoming links, excluding "index")
	orphans := make([]string, 0)
	for pageName := range idx.links {
		if pageName == "index" {
			continue
		}
		if len(idx.backlinks[pageName]) == 0 {
			orphans = append(orphans, pageName)
		}
	}
	slices.Sort(orphans)

	// Find broken links (target does not exist as a page)
	broken := make([]brokenLink, 0)
	for source, targets := range idx.links {
		for _, target := range targets {
			if strings.Contains(target, "://") {
				continue
			}
			// Strip fragment (e.g. "page#section" → "page")
			if i := strings.IndexByte(target, '#'); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			if _, exists := idx.links[target]; !exists {
				broken = append(broken, brokenLink{Source: source, Target: target})
			}
		}
	}
	slices.SortFunc(broken, func(a, b brokenLink) int {
		if c := strings.Compare(a.Source, b.Source); c != 0 {
			return c
		}
		return strings.Compare(a.Target, b.Target)
	})

	stats := graphStats{
		TotalPages:      int(idx.pages),
		TotalLinks:      totalLinks,
		OrphanCount:     len(orphans),
		BrokenLinkCount: len(broken),
	}

	switch *output {
	case "json":
		pages := make(map[string]graphPage, len(idx.links))
		for pageName, targets := range idx.links {
			outgoing := targets
			if outgoing == nil {
				outgoing = make([]string, 0)
			}
			incoming := idx.backlinks[pageName]
			if incoming == nil {
				incoming = make([]string, 0)
			}
			pages[pageName] = graphPage{
				Outgoing: outgoing,
				Incoming: incoming,
			}
		}
		out := graphOutput{
			Pages:       pages,
			Orphans:     orphans,
			BrokenLinks: broken,
			Stats:       stats,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)

	case "text":
		log.Printf("indexed %d pages (%d links) in %.2fs\n", idx.pages, totalLinks, elapsed.Seconds())

		if len(orphans) > 0 {
			fmt.Println()
			fmt.Println("orphan pages (no incoming links):")
			for _, o := range orphans {
				fmt.Printf("  %s\n", o)
			}
		}

		if len(broken) > 0 {
			fmt.Println()
			fmt.Println("broken links (target does not exist):")
			for _, bl := range broken {
				fmt.Printf("  %s → %s\n", bl.Source, bl.Target)
			}
		}

		fmt.Println()
		fmt.Println("graph summary:")
		fmt.Printf("  pages:        %d\n", stats.TotalPages)
		fmt.Printf("  links:        %d\n", stats.TotalLinks)
		fmt.Printf("  orphans:      %d\n", stats.OrphanCount)
		fmt.Printf("  broken links: %d\n", stats.BrokenLinkCount)

	default:
		return fmt.Errorf("unknown output format %q (supported: text, json)", *output)
	}

	return nil
}
