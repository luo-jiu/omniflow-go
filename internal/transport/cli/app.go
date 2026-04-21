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
	Flags    []string
	Examples []string
	Run      func(args []string) error
	Children map[string]*command
}

type helpOptions struct {
	showExamples     bool
	examplesExplicit bool
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
		a.printCommandHelpTo(a.root, a.stderr, resolvedHelpOptions(a.root, helpOptions{}))
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
				a.printCommandHelpTo(current, a.stderr, resolvedHelpOptions(current, helpOptions{}))
				return 1
			}
			if err := current.Run(nil); err != nil {
				return a.failErr(err)
			}
			return 0
		}

		if isHelpFlag(remaining[0]) {
			opts, err := parseHelpOptions(remaining[1:])
			if err != nil {
				return a.failErr(err)
			}
			a.printCommandHelpTo(current, a.stdout, resolvedHelpOptions(current, opts))
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
			a.printCommandHelpTo(current, a.stderr, resolvedHelpOptions(current, helpOptions{}))
			return 1
		}

		current = next
		remaining = remaining[1:]
	}
}

func (a *App) runHelp(tokens []string) int {
	path, optionTokens, err := splitHelpPathAndOptions(tokens)
	if err != nil {
		return a.failErr(err)
	}

	current := a.root
	for _, token := range path {
		next, ok := current.Children[token]
		if !ok {
			a.failf("unknown command path: %s", strings.Join(path, " "))
			a.printCommandHelpTo(a.root, a.stderr, resolvedHelpOptions(a.root, helpOptions{}))
			return 1
		}
		current = next
	}

	opts, err := parseHelpOptions(optionTokens)
	if err != nil {
		return a.failErr(err)
	}
	a.printCommandHelpTo(current, a.stdout, resolvedHelpOptions(current, opts))
	return 0
}

func (a *App) buildCommandTree() *command {
	root := &command{
		Name:    "command",
		Summary: "OmniFlow CLI",
		Usage:   "of <command> [subcommand] [flags]",
		Examples: []string{
			"of help",
			"of help fs",
			"of help fs mkdir --examples",
			"of health",
			"of auth login --username demo --password demo",
		},
		Children: map[string]*command{},
	}

	root.Children["health"] = &command{
		Name:    "health",
		Summary: "Check service health status",
		Usage:   "of health [--base-url <url>] [--json]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of health",
			"of health --json",
		},
		Run: a.runHealth,
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
		Flags: []string{
			"--username <name>   login username (required)",
			"--password <pwd>    login password (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of auth login --username Loyce --password 123456",
			"of auth login --username Loyce --password 123456 --json",
		},
		Run: a.runAuthLogin,
	}
	auth.Children["status"] = &command{
		Name:    "status",
		Summary: "Check whether current local session is still valid",
		Usage:   "of auth status [--base-url <url>] [--json]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of auth status",
			"of auth status --json",
		},
		Run: a.runAuthStatus,
	}
	auth.Children["whoami"] = &command{
		Name:    "whoami",
		Summary: "Get current user information with local session",
		Usage:   "of auth whoami [--base-url <url>] [--json]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of auth whoami",
			"of auth whoami --json",
		},
		Run: a.runAuthWhoAmI,
	}
	auth.Children["logout"] = &command{
		Name:    "logout",
		Summary: "Logout current local session",
		Usage:   "of auth logout [--base-url <url>] [--dry-run]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
		},
		Examples: []string{
			"of auth logout",
			"of auth logout --dry-run",
		},
		Run: a.runAuthLogout,
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
		Flags: []string{
			"--last-id <id>      pagination cursor",
			"--size <n>          page size",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of lib ls --size 20",
			"of lib ls --last-id 100 --size 20 --json",
		},
		Run: a.runLibraryList,
	}
	root.Children["lib"] = lib

	fs := &command{
		Name:     "fs",
		Summary:  "File system commands",
		Usage:    "of fs <mkdir|rename|mv|rm|ls|search|archive|recycle|path> [flags]",
		Children: map[string]*command{},
	}
	fs.Children["mkdir"] = &command{
		Name:    "mkdir",
		Summary: "Create a directory node",
		Usage:   "of fs mkdir --library-id <id> --name <name> [--parent-id <id>|--parent-path </a/b>] [--conflict-policy <error|auto_rename>] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--name <name>       directory name (required)",
			"--parent-id <id>    parent node id, defaults to root",
			"--parent-path <p>   parent path from root",
			"--conflict-policy   name conflict strategy: error or auto_rename",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs mkdir --library-id 1 --name docs",
			"of fs mkdir --library-id 1 --parent-path /docs --name chapter-1",
			"of fs mkdir --library-id 1 --name docs --conflict-policy auto_rename",
			"of fs mkdir --library-id 1 --name docs --dry-run --json",
			"of fs mkdir --library-id 1 --parent-id 100 --name chapter-1 --json",
		},
		Run: a.runFSMkdir,
	}
	fs.Children["rename"] = &command{
		Name:    "rename",
		Summary: "Rename a node",
		Usage:   "of fs rename --node-id <id> --name <name> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--node-id <id>      target node id (required)",
			"--name <name>       new node name (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs rename --node-id 123 --name notes",
			"of fs rename --node-id 123 --name notes --dry-run --json",
			"of fs rename --node-id 123 --name notes-v2 --json",
		},
		Run: a.runFSRename,
	}
	fs.Children["mv"] = &command{
		Name:    "mv",
		Summary: "Move a node to another parent",
		Usage:   "of fs mv --library-id <id> (--node-id <id>|--node-path </a/b>) (--new-parent-id <id>|--new-parent-path </a/b>) [--before-node-id <id>] [--name <name>] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>    library id (required)",
			"--node-id <id>       target node id",
			"--node-path <p>      target node path from root",
			"--new-parent-id <id> target parent node id",
			"--new-parent-path <p> target parent path from root",
			"--before-node-id <id> optional sibling id to place before",
			"--name <name>        optional rename while moving",
			"--base-url <url>     API base URL",
			"--dry-run            preview only, do not commit changes",
			"--json               output JSON",
		},
		Examples: []string{
			"of fs mv --library-id 1 --node-id 123 --new-parent-id 200",
			"of fs mv --library-id 1 --node-path /docs/a.md --new-parent-path /archive",
			"of fs mv --library-id 1 --node-id 123 --new-parent-id 200 --dry-run --json",
			"of fs mv --library-id 1 --node-id 123 --new-parent-id 200 --before-node-id 201 --json",
		},
		Run: a.runFSMove,
	}
	fs.Children["rm"] = &command{
		Name:    "rm",
		Summary: "Move a node tree to recycle bin",
		Usage:   "of fs rm --library-id <id> (--node-id <id>|--path </a/b>) [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--node-id <id>      target node id",
			"--path <path>       target node path from root",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs rm --library-id 1 --node-id 123",
			"of fs rm --library-id 1 --path /docs/a.md",
			"of fs rm --library-id 1 --node-id 123 --dry-run --json",
			"of fs rm --library-id 1 --node-id 123 --json",
		},
		Run: a.runFSRemove,
	}
	fs.Children["ls"] = &command{
		Name:    "ls",
		Summary: "List direct children under a node",
		Usage:   "of fs ls --library-id <id> --node-id <id> [--base-url <url>] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--node-id <id>      node id (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs ls --library-id 1 --node-id 10",
			"of fs ls --library-id 1 --node-id 10 --json",
		},
		Run: a.runFSList,
	}
	fs.Children["search"] = &command{
		Name:    "search",
		Summary: "Search nodes by keyword/tag constraints",
		Usage:   "of fs search --library-id <id> [--keyword <kw>] [--tag-ids 1,2] [--tag-match-mode ANY|ALL] [--limit <n>] [--base-url <url>] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--keyword <kw>      node name keyword",
			"--tag-ids <list>    comma-separated tag ids",
			"--tag-match-mode    ANY or ALL",
			"--limit <n>         max result size",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs search --library-id 1 --keyword contract",
			"of fs search --library-id 1 --tag-ids 1,2 --tag-match-mode ALL --json",
		},
		Run: a.runFSSearch,
	}
	archive := &command{
		Name:     "archive",
		Summary:  "Archive directory commands",
		Usage:    "of fs archive <batch-set-built-in-type> [flags]",
		Children: map[string]*command{},
	}
	archive.Children["batch-set-built-in-type"] = &command{
		Name:    "batch-set-built-in-type",
		Summary: "Batch set direct child directory built-in type by archive parent",
		Usage:   "of fs archive batch-set-built-in-type --node-id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--node-id <id>      archive directory node id (required, >0)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs archive batch-set-built-in-type --node-id 123",
			"of fs archive batch-set-built-in-type --node-id 123 --dry-run --json",
		},
		Run: a.runFSArchiveBatchSetBuiltInType,
	}
	fs.Children["archive"] = archive
	recycle := &command{
		Name:     "recycle",
		Summary:  "Recycle bin commands",
		Usage:    "of fs recycle <ls|clear|restore|hard> [flags]",
		Children: map[string]*command{},
	}
	recycle.Children["ls"] = &command{
		Name:    "ls",
		Summary: "List top-level nodes in recycle bin",
		Usage:   "of fs recycle ls --library-id <id> [--base-url <url>] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs recycle ls --library-id 1",
			"of fs recycle ls --library-id 1 --json",
		},
		Run: a.runFSRecycleList,
	}
	recycle.Children["clear"] = &command{
		Name:    "clear",
		Summary: "Permanently clear all top-level nodes in recycle bin",
		Usage:   "of fs recycle clear --library-id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs recycle clear --library-id 1",
			"of fs recycle clear --library-id 1 --dry-run --json",
			"of fs recycle clear --library-id 1 --json",
		},
		Run: a.runFSRecycleClear,
	}
	recycle.Children["restore"] = &command{
		Name:    "restore",
		Summary: "Restore a node tree from recycle bin",
		Usage:   "of fs recycle restore --library-id <id> --node-id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--node-id <id>      target node id in recycle bin (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs recycle restore --library-id 1 --node-id 123",
			"of fs recycle restore --library-id 1 --node-id 123 --dry-run --json",
			"of fs recycle restore --library-id 1 --node-id 123 --json",
		},
		Run: a.runFSRecycleRestore,
	}
	recycle.Children["hard"] = &command{
		Name:    "hard",
		Summary: "Permanently delete a node tree from recycle bin",
		Usage:   "of fs recycle hard --library-id <id> --node-id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--node-id <id>      target node id in recycle bin (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs recycle hard --library-id 1 --node-id 123",
			"of fs recycle hard --library-id 1 --node-id 123 --dry-run --json",
			"of fs recycle hard --library-id 1 --node-id 123 --json",
		},
		Run: a.runFSRecycleHardDelete,
	}
	fs.Children["recycle"] = recycle
	pathCmd := &command{
		Name:     "path",
		Summary:  "Path helper commands",
		Usage:    "of fs path <resolve> [flags]",
		Children: map[string]*command{},
	}
	pathCmd.Children["resolve"] = &command{
		Name:    "resolve",
		Summary: "Resolve a node path to node id",
		Usage:   "of fs path resolve --library-id <id> --path </a/b> [--base-url <url>] [--json]",
		Flags: []string{
			"--library-id <id>   library id (required)",
			"--path <path>       path from root, e.g. /docs/ch1 (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of fs path resolve --library-id 1 --path /docs/ch1",
			"of fs path resolve --library-id 1 --path docs/ch1 --json",
		},
		Run: a.runFSPathResolve,
	}
	fs.Children["path"] = pathCmd
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
		Flags: []string{
			"--json              output JSON",
		},
		Examples: []string{
			"of config show",
			"of config show --json",
		},
		Run: a.runConfigShow,
	}
	root.Children["config"] = config

	browserMap := &command{
		Name:     "browser-map",
		Summary:  "Browser file mapping commands",
		Usage:    "of browser-map <ls|resolve|create|update|rm> [flags]",
		Children: map[string]*command{},
	}
	browserMap.Children["ls"] = &command{
		Name:    "ls",
		Summary: "List browser file mappings",
		Usage:   "of browser-map ls [--base-url <url>] [--json]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-map ls",
			"of browser-map ls --json",
		},
		Run: a.runBrowserMapList,
	}
	browserMap.Children["resolve"] = &command{
		Name:    "resolve",
		Summary: "Resolve browser mapping by file extension",
		Usage:   "of browser-map resolve --ext <ext> [--base-url <url>] [--json]",
		Flags: []string{
			"--ext <ext>         file extension without leading dot (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-map resolve --ext excalidraw",
			"of browser-map resolve --ext txt --json",
		},
		Run: a.runBrowserMapResolve,
	}
	browserMap.Children["create"] = &command{
		Name:    "create",
		Summary: "Create a browser file mapping",
		Usage:   "of browser-map create --ext <ext> --url <url> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--ext <ext>         file extension without leading dot (required)",
			"--url <url>         site url (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-map create --ext excalidraw --url https://excalidraw.com",
			"of browser-map create --ext txt --url https://example.test --dry-run --json",
		},
		Run: a.runBrowserMapCreate,
	}
	browserMap.Children["update"] = &command{
		Name:    "update",
		Summary: "Update a browser file mapping",
		Usage:   "of browser-map update --id <id> --ext <ext> --url <url> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--id <id>           mapping id (required)",
			"--ext <ext>         file extension without leading dot (required)",
			"--url <url>         site url (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-map update --id 3 --ext excalidraw --url https://excalidraw.com",
			"of browser-map update --id 3 --ext txt --url https://example.test --dry-run --json",
		},
		Run: a.runBrowserMapUpdate,
	}
	browserMap.Children["rm"] = &command{
		Name:    "rm",
		Summary: "Delete a browser file mapping",
		Usage:   "of browser-map rm --id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--id <id>           mapping id (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-map rm --id 3",
			"of browser-map rm --id 3 --dry-run --json",
		},
		Run: a.runBrowserMapDelete,
	}
	root.Children["browser-map"] = browserMap

	browserBookmark := &command{
		Name:     "browser-bookmark",
		Summary:  "Browser bookmark commands",
		Usage:    "of browser-bookmark <tree|match|create|update|move|rm> [flags]",
		Children: map[string]*command{},
	}
	browserBookmark.Children["tree"] = &command{
		Name:    "tree",
		Summary: "Show browser bookmark tree",
		Usage:   "of browser-bookmark tree [--base-url <url>] [--json]",
		Flags: []string{
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark tree",
			"of browser-bookmark tree --json",
		},
		Run: a.runBrowserBookmarkTree,
	}
	browserBookmark.Children["match"] = &command{
		Name:    "match",
		Summary: "Match a browser url against saved bookmarks",
		Usage:   "of browser-bookmark match --url <url> [--base-url <url>] [--json]",
		Flags: []string{
			"--url <url>         browser url to match (required)",
			"--base-url <url>    API base URL",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark match --url https://example.com/path?utm=1",
			"of browser-bookmark match --url https://example.com/path --json",
		},
		Run: a.runBrowserBookmarkMatch,
	}
	browserBookmark.Children["create"] = &command{
		Name:    "create",
		Summary: "Create a browser bookmark or folder",
		Usage:   "of browser-bookmark create --title <title> [--kind <url|folder>] [--url <url>] [--parent-id <id>] [--icon-url <url>] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--title <title>     bookmark title (required)",
			"--kind <kind>       bookmark kind: url or folder",
			"--url <url>         bookmark url for url kind",
			"--parent-id <id>    parent folder id",
			"--icon-url <url>    bookmark icon url",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark create --title Example --url https://example.com",
			"of browser-bookmark create --title Work --kind folder --dry-run --json",
		},
		Run: a.runBrowserBookmarkCreate,
	}
	browserBookmark.Children["update"] = &command{
		Name:    "update",
		Summary: "Update a browser bookmark",
		Usage:   "of browser-bookmark update --id <id> [--title <title>] [--url <url>] [--icon-url <url>] [--clear-icon] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--id <id>           bookmark id (required)",
			"--title <title>     bookmark title",
			"--url <url>         bookmark url",
			"--icon-url <url>    bookmark icon url",
			"--clear-icon        clear bookmark icon",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark update --id 8 --title Example Home",
			"of browser-bookmark update --id 8 --clear-icon --dry-run --json",
		},
		Run: a.runBrowserBookmarkUpdate,
	}
	browserBookmark.Children["move"] = &command{
		Name:    "move",
		Summary: "Move a browser bookmark within the tree",
		Usage:   "of browser-bookmark move --id <id> [--parent-id <id>] [--before-id <id>|--after-id <id>] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--id <id>           bookmark id (required)",
			"--parent-id <id>    target parent folder id",
			"--before-id <id>    insert before sibling id",
			"--after-id <id>     insert after sibling id",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark move --id 8 --parent-id 2 --after-id 5",
			"of browser-bookmark move --id 8 --before-id 3 --dry-run --json",
		},
		Run: a.runBrowserBookmarkMove,
	}
	browserBookmark.Children["rm"] = &command{
		Name:    "rm",
		Summary: "Delete a browser bookmark subtree",
		Usage:   "of browser-bookmark rm --id <id> [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--id <id>           bookmark id (required)",
			"--base-url <url>    API base URL",
			"--dry-run           preview only, do not commit changes",
			"--json              output JSON",
		},
		Examples: []string{
			"of browser-bookmark rm --id 8",
			"of browser-bookmark rm --id 8 --dry-run --json",
		},
		Run: a.runBrowserBookmarkDelete,
	}
	browserBookmark.Children["import"] = &command{
		Name:    "import",
		Summary: "Import browser bookmarks from a normalized JSON tree",
		Usage:   "of browser-bookmark import --file <path> [--source <label>] [--base-url <url>] [--dry-run] [--json]",
		Flags: []string{
			"--file <path>      JSON file path containing import payload (required)",
			"--source <label>   import source label",
			"--base-url <url>   API base URL",
			"--dry-run          preview only, do not commit changes",
			"--json             output JSON",
		},
		Examples: []string{
			"of browser-bookmark import --file ./browser-bookmarks.json --source chrome-local",
			"of browser-bookmark import --file ./browser-bookmarks.json --dry-run --json",
		},
		Run: a.runBrowserBookmarkImport,
	}
	root.Children["browser-bookmark"] = browserBookmark

	return root
}

func (a *App) printCommandHelpTo(cmd *command, out io.Writer, options helpOptions) {
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

	if len(cmd.Flags) > 0 {
		fmt.Fprintln(out, "\nFlags:")
		for _, line := range cmd.Flags {
			if strings.TrimSpace(line) == "" {
				continue
			}
			fmt.Fprintf(out, "  %s\n", line)
		}
	}

	if len(cmd.Examples) > 0 && options.showExamples {
		fmt.Fprintln(out, "\nExamples:")
		for _, example := range cmd.Examples {
			if strings.TrimSpace(example) == "" {
				continue
			}
			fmt.Fprintf(out, "  %s\n", example)
		}
	} else if len(cmd.Examples) > 0 && len(cmd.Children) == 0 {
		fmt.Fprintln(out, "\nTip:")
		fmt.Fprintln(out, "  Append `--examples` to show command examples.")
	}
}

func resolvedHelpOptions(cmd *command, raw helpOptions) helpOptions {
	resolved := helpOptions{
		showExamples: len(cmd.Children) > 0,
	}
	if raw.examplesExplicit {
		resolved.showExamples = raw.showExamples
	}
	return resolved
}

func splitHelpPathAndOptions(tokens []string) ([]string, []string, error) {
	path := make([]string, 0, len(tokens))
	optionTokens := make([]string, 0, 2)
	optionsStarted := false

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		if strings.HasPrefix(token, "-") {
			optionsStarted = true
			optionTokens = append(optionTokens, token)
			continue
		}

		if optionsStarted {
			return nil, nil, fmt.Errorf("unexpected help path token after options: %s", token)
		}
		path = append(path, token)
	}

	return path, optionTokens, nil
}

func parseHelpOptions(tokens []string) (helpOptions, error) {
	opts := helpOptions{}
	for _, token := range tokens {
		switch strings.TrimSpace(token) {
		case "", "-h", "--help":
			continue
		case "--examples", "-x", "--verbose", "-v":
			opts.showExamples = true
			opts.examplesExplicit = true
		default:
			return helpOptions{}, fmt.Errorf("unknown help option: %s", token)
		}
	}
	return opts, nil
}

func isHelpFlag(token string) bool {
	switch token {
	case "-h", "--help":
		return true
	default:
		return false
	}
}
