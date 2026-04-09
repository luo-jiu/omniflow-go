package cli

import (
	"context"
	"errors"
	"strings"
)

func (a *App) runAuthLogin(args []string) error {
	fs := a.newFlagSet("auth login")

	var (
		baseURL  string
		username string
		password string
		jsonOut  bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&username, "username", "", "login username")
	fs.StringVar(&password, "password", "", "login password")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return errors.New("`--username` and `--password` are required")
	}

	session, client, err := a.resolveClient(baseURL, false)
	if err != nil {
		return err
	}

	loginResult, err := client.Login(context.Background(), username, password)
	if err != nil {
		return err
	}

	session.Username = strings.TrimSpace(loginResult.Username)
	if session.Username == "" {
		session.Username = username
	}
	session.Token = strings.TrimSpace(loginResult.Token)
	if err := SaveSession(session); err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(loginResult)
	}

	a.printf("login success: username=%s user_id=%d\n", session.Username, loginResult.UserInfo.ID)
	return nil
}

func (a *App) runAuthStatus(args []string) error {
	fs := a.newFlagSet("auth status")

	var (
		baseURL string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	session, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	ok, err := client.AuthStatus(context.Background())
	if err != nil {
		return err
	}

	result := map[string]any{
		"baseUrl":  session.BaseURL,
		"username": session.Username,
		"loggedIn": ok,
	}
	if jsonOut {
		if err := a.printJSON(result); err != nil {
			return err
		}
		if !ok {
			return errors.New("current session is not logged in")
		}
		return nil
	}

	a.printf("logged_in=%t username=%s base_url=%s\n", ok, session.Username, session.BaseURL)
	if !ok {
		return errors.New("current session is not logged in")
	}
	return nil
}

func (a *App) runAuthWhoAmI(args []string) error {
	fs := a.newFlagSet("auth whoami")

	var (
		baseURL string
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
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

	user, err := client.WhoAmI(context.Background())
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(user)
	}

	a.printf("id=%d username=%s nickname=%s status=%s\n", user.ID, user.Username, user.Nickname, user.Status)
	return nil
}

func (a *App) runAuthLogout(args []string) error {
	fs := a.newFlagSet("auth logout")

	var baseURL string
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	session, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if err := client.Logout(context.Background()); err != nil {
		return err
	}

	session.Username = ""
	session.Token = ""
	if err := SaveSession(session); err != nil {
		return err
	}

	a.println("logout success")
	return nil
}
