package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type command struct {
	Name     string
	Summary  string
	Usage    string
	Examples []string
	Run      func(args []string) error
	Children map[string]*command
}

type App struct {
	stdout io.Writer
	stderr io.Writer
	root   *command
}

func NewApp(stdout, stderr io.Writer) *App {
	app := &App{
		stdout: stdout,
		stderr: stderr,
	}
	app.root = app.buildCommandTree()
	return app
}

func Run(args []string, stdout, stderr io.Writer) int {
	return NewApp(stdout, stderr).Run(args)
}

func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.printCommandHelpTo(a.root, a.stderr)
		return 1
	}

	if args[0] == "help" {
		return a.runHelp(args[1:])
	}

	current := a.root
	remaining := args
	for {
		if len(remaining) == 0 {
			if current.Run == nil {
				a.printCommandHelpTo(current, a.stderr)
				return 1
			}
			if err := current.Run(nil); err != nil {
				return a.failErr(err)
			}
			return 0
		}

		if isHelpFlag(remaining[0]) {
			a.printCommandHelpTo(current, a.stdout)
			return 0
		}

		// Leaf command: forward all remaining args to the handler.
		if len(current.Children) == 0 {
			if current.Run == nil {
				a.failf("command is not executable: %s", strings.TrimSpace(current.Name))
				return 1
			}
			if err := current.Run(remaining); err != nil {
				return a.failErr(err)
			}
			return 0
		}

		next, ok := current.Children[remaining[0]]
		if !ok {
			if current == a.root {
				a.failf("unknown command: %s", remaining[0])
			} else {
				a.failf("unknown %s command: %s", current.Name, remaining[0])
			}
			a.printCommandHelpTo(current, a.stderr)
			return 1
		}

		current = next
		remaining = remaining[1:]
	}
}

func (a *App) runHelp(path []string) int {
	current := a.root
	for _, token := range path {
		if token == "" {
			continue
		}
		next, ok := current.Children[token]
		if !ok {
			a.failf("unknown command path: %s", strings.Join(path, " "))
			a.printCommandHelpTo(a.root, a.stderr)
			return 1
		}
		current = next
	}
	a.printCommandHelpTo(current, a.stdout)
	return 0
}

func (a *App) buildCommandTree() *command {
	root := &command{
		Name:    "command",
		Summary: "OmniFlow CLI",
		Usage:   "of <command> [subcommand] [flags]",
		Examples: []string{
			"of health",
			"of auth login --username demo --password demo",
			"of auth whoami",
			"of lib ls --size 20",
			"of fs search --library-id 1 --keyword contract --limit 20",
		},
		Children: map[string]*command{},
	}

	root.Children["health"] = &command{
		Name:    "health",
		Summary: "Check service health status",
		Usage:   "of health [--base-url <url>] [--json]",
		Run:     a.runHealth,
	}

	auth := &command{
		Name:     "auth",
		Summary:  "Authentication commands",
		Usage:    "of auth <login|status|whoami|logout> [flags]",
		Children: map[string]*command{},
	}
	auth.Children["login"] = &command{
		Name:    "login",
		Summary: "Login with username and password",
		Usage:   "of auth login --username <name> --password <password> [--base-url <url>] [--json]",
		Run:     a.runAuthLogin,
	}
	auth.Children["status"] = &command{
		Name:    "status",
		Summary: "Check whether current local session is still valid",
		Usage:   "of auth status [--base-url <url>] [--json]",
		Run:     a.runAuthStatus,
	}
	auth.Children["whoami"] = &command{
		Name:    "whoami",
		Summary: "Get current user information with local session",
		Usage:   "of auth whoami [--base-url <url>] [--json]",
		Run:     a.runAuthWhoAmI,
	}
	auth.Children["logout"] = &command{
		Name:    "logout",
		Summary: "Logout current local session",
		Usage:   "of auth logout [--base-url <url>]",
		Run:     a.runAuthLogout,
	}
	root.Children["auth"] = auth

	lib := &command{
		Name:     "lib",
		Summary:  "Library commands",
		Usage:    "of lib <ls> [flags]",
		Children: map[string]*command{},
	}
	lib.Children["ls"] = &command{
		Name:    "ls",
		Summary: "List libraries of current user",
		Usage:   "of lib ls [--last-id <id>] [--size <n>] [--base-url <url>] [--json]",
		Run:     a.runLibraryList,
	}
	root.Children["lib"] = lib

	fs := &command{
		Name:     "fs",
		Summary:  "File system commands",
		Usage:    "of fs <ls|search> [flags]",
		Children: map[string]*command{},
	}
	fs.Children["ls"] = &command{
		Name:    "ls",
		Summary: "List direct children under a node",
		Usage:   "of fs ls --library-id <id> --node-id <id> [--base-url <url>] [--json]",
		Run:     a.runFSList,
	}
	fs.Children["search"] = &command{
		Name:    "search",
		Summary: "Search nodes by keyword/tag constraints",
		Usage:   "of fs search --library-id <id> [--keyword <kw>] [--tag-ids 1,2] [--tag-match-mode ANY|ALL] [--limit <n>] [--base-url <url>] [--json]",
		Run:     a.runFSSearch,
	}
	root.Children["fs"] = fs

	config := &command{
		Name:     "config",
		Summary:  "CLI local config commands",
		Usage:    "of config <show> [flags]",
		Children: map[string]*command{},
	}
	config.Children["show"] = &command{
		Name:    "show",
		Summary: "Show merged config/session snapshot",
		Usage:   "of config show [--json]",
		Run:     a.runConfigShow,
	}
	root.Children["config"] = config

	return root
}

func (a *App) printCommandHelpTo(cmd *command, out io.Writer) {
	if cmd == nil {
		return
	}
	if out == nil {
		out = a.stderr
	}

	if strings.TrimSpace(cmd.Summary) != "" {
		fmt.Fprintf(out, "%s\n\n", strings.TrimSpace(cmd.Summary))
	}

	if usage := strings.TrimSpace(cmd.Usage); usage != "" {
		fmt.Fprintf(out, "Usage:\n  %s\n", usage)
	}

	if len(cmd.Children) > 0 {
		keys := make([]string, 0, len(cmd.Children))
		for key := range cmd.Children {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		fmt.Fprintln(out, "\nCommands:")
		for _, key := range keys {
			child := cmd.Children[key]
			if child == nil {
				continue
			}
			fmt.Fprintf(out, "  %-12s %s\n", key, strings.TrimSpace(child.Summary))
		}
	}

	if len(cmd.Examples) > 0 {
		fmt.Fprintln(out, "\nExamples:")
		for _, example := range cmd.Examples {
			if strings.TrimSpace(example) == "" {
				continue
			}
			fmt.Fprintf(out, "  %s\n", example)
		}
	}
}

func isHelpFlag(token string) bool {
	switch token {
	case "-h", "--help":
		return true
	default:
		return false
	}
}
