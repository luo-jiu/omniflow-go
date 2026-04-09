package cli

import (
	"context"
)

func (a *App) runLibraryList(args []string) error {
	fs := a.newFlagSet("lib ls")

	var (
		baseURL string
		lastID  uint64
		size    int
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&lastID, "last-id", 0, "scroll cursor id")
	fs.IntVar(&size, "size", 10, "page size")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	result, err := client.ScrollLibraries(context.Background(), lastID, size)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(result)
	}

	if len(result.Items) == 0 {
		a.println("(no libraries)")
		return nil
	}
	for _, item := range result.Items {
		a.printf("id=%d name=%s starred=%t\n", item.ID, item.Name, item.Starred)
	}
	a.printf("has_more=%t\n", result.HasMore)
	return nil
}
