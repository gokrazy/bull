package bull

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"

	"github.com/BurntSushi/toml"
	"github.com/gokrazy/bull"
)

const (
	bullPrefix    = "_bull/"
	bullURLPrefix = "/_bull/"
)

func defaultContentDir() string {
	if v := os.Getenv("BULL_CONTENT"); v != "" {
		return v
	}
	dir, _ := os.Getwd()
	// If Getwd failed, return the empty string.
	// That still means “current working directory”,
	// we just have no name to display to the user.
	return dir
}

func loadContentSettings(content *os.Root) (bull.ContentSettings, error) {
	cs := bull.ContentSettings{
		HardWraps: true, // like SilverBullet
	}
	csf, err := content.Open("_bull/content-settings.toml")
	if err != nil {
		if os.IsNotExist(err) {
			// Start with default settings if no content-settings.toml exists.
			return cs, nil
		}
		return cs, err
	}
	defer csf.Close()
	csb, err := io.ReadAll(csf)
	if err != nil {
		return cs, err
	}
	if err := toml.Unmarshal(csb, &cs); err != nil {
		return cs, err
	}
	log.Printf("bull content settings loaded from %s", csf.Name())
	return cs, nil
}

var (
	contentDir = flag.String("content",
		defaultContentDir(),
		"content directory. bull considers each markdown file in this directory a page and will only serve files from this directory")
)

func usage(fset *flag.FlagSet, help string) func() {
	return func() {
		fmt.Fprintf(fset.Output(), "%s", help)
		fmt.Fprintf(fset.Output(), "\nCommand-line flags:\n")
		fset.PrintDefaults()
		fmt.Fprintf(fset.Output(), "\n(See bull -help for global command-line flags.)\n")
	}
}

type Customization struct {
	// AfterPageRead is a hook that is called after a page is read and can be
	// used to modify or replace the content.
	AfterPageRead func([]byte) []byte

	// TODO: markdown goldmark option hook?
}

func (c *Customization) Runbull() error {
	info, ok := debug.ReadBuildInfo()
	mainVersion := info.Main.Version
	if !ok {
		mainVersion = "<runtime/debug.ReadBuildInfo failed>"
	}
	fmt.Printf("github.com/gokrazy/bull %s\n", mainVersion)

	flag.Usage = func() {
		os.Stderr.Write([]byte(`
bull is a minimalistic bullet journaling software.

Syntax: bull [global flags] <verb> [flags] [args]

To get help on any verb, use bull <verb> -help or bull help <verb>.

If no verb is specified, bull will default to 'serve'.

Verbs:
  serve  - serve markdown pages
  mv     - rename markdown page and update links

Examples:
  % bull                               # serve the current directory
  % bull -content ~/keep serve         # serve ~/keep
  % bull serve -listen=100.5.23.42:80  # serve on a Tailscale VPN IP

Command-line flags:
`))
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		// no args? default to 'serve' verb
		args = []string{"serve"}
	}
	verb, args := args[0], args[1:]
	if verb == "help" {
		// Turn bull help <verb> into bull <verb> -help
		if len(args) == 0 {
			flag.Usage()
			os.Exit(2)
		}
		verb = args[0]
		args = []string{"-help"}
	}
	switch verb {
	case "serve", "server":
		return c.serve(args)
	case "mv":
		return mv(args)
	}
	fmt.Fprintf(os.Stderr, "unknown verb %q\n", verb)
	flag.Usage()
	os.Exit(2)
	return nil
}
