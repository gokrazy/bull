package bull

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func (b *bullServer) itasklistAPI(w http.ResponseWriter, r *http.Request) error {
	if b.editor == "" {
		return httpError(http.StatusForbidden, fmt.Errorf("running in read-only mode (-editor= flag)"))
	}
	src := r.PathValue("page")
	lineStr := r.FormValue("checkbox-line")
	if lineStr == "" {
		return fmt.Errorf("invalid request: no ?checkbox-line parameter")
	}
	line, err := strconv.ParseInt(lineStr, 0, 64)
	if err != nil {
		return err
	}
	log.Printf("toggling checkbox (line=%d) on page=%q", line, src)
	possibilities := page2files(src)
	if isMarkdown(src) {
		possibilities = []string{src}
	}
	pg, err := b.readFirst(possibilities)
	if err != nil {
		return err
	}

	updatedContent := toggleCheckbox(pg.DiskContent, int(line))
	if updatedContent != pg.DiskContent {
		f, err := b.content.OpenFile(pg.FileName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write([]byte(updatedContent)); err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}

	http.Redirect(w, r, b.root+pg.URLPath(), http.StatusFound)
	return nil
}

// like the regexp in itasklist.go, but not anchored
var taskListRegexp = regexp.MustCompile(`\[([\sxX])\]\s*`)

func toggleCheckbox(content string, checkboxLine int) string {
	lines := strings.Split(content, "\n")
	if checkboxLine-1 > len(lines)-1 {
		return content // checkbox line out of range
	}
	line := lines[checkboxLine-1]
	m := taskListRegexp.FindStringSubmatchIndex(line)
	if m == nil {
		return content // line contains no checkbox
	}
	before := line[:m[2]]
	after := line[m[3]:]
	checked := line[m[2]:m[3]]
	opposite := "x"
	if checked == "x" || checked == "X" {
		opposite = " "
	}
	lines[checkboxLine-1] = before + opposite + after
	// TODO: move the checkbox into the section it belongs to
	// (ticked / un-ticked parts of the list)
	return strings.Join(lines, "\n")
}
