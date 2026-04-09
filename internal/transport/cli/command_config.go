package cli

func (a *App) runConfigShow(args []string) error {
	fs := a.newFlagSet("config show")

	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	session, err := ResolveSession("")
	if err != nil {
		return err
	}
	configPath, err := ConfigFilePath()
	if err != nil {
		return err
	}

	response := map[string]any{
		"configPath": configPath,
		"baseUrl":    session.BaseURL,
		"username":   session.Username,
		"tokenSet":   session.Token != "",
	}
	if jsonOut {
		return a.printJSON(response)
	}

	a.printf("config=%s\nbase_url=%s\nusername=%s\ntoken_set=%t\n",
		configPath, session.BaseURL, session.Username, session.Token != "")
	return nil
}
