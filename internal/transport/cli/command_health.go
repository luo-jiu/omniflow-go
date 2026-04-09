package cli

import (
	"context"
)

func (a *App) runHealth(args []string) error {
	fs := a.newFlagSet("health")

	var (
		baseURL string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url, e.g. http://127.0.0.1:8850")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	session, client, err := a.resolveClient(baseURL, false)
	if err != nil {
		return err
	}
	status, err := client.Health(context.Background())
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(status)
	}

	a.printf("status=%s service=%s env=%s version=%s time=%s\n",
		status.Status, status.Name, status.Env, status.Version, status.Timestamp.Format("2006-01-02 15:04:05Z07:00"))
	if session.BaseURL != "" {
		a.printf("base_url=%s\n", session.BaseURL)
	}
	return nil
}
