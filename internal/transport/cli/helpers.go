package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (a *App) newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func (a *App) printJSON(v any) error {
	encoder := json.NewEncoder(a.stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func (a *App) printf(format string, args ...any) {
	fmt.Fprintf(a.stdout, format, args...)
}

func (a *App) println(args ...any) {
	fmt.Fprintln(a.stdout, args...)
}

func (a *App) failErr(err error) int {
	fmt.Fprintf(a.stderr, "error: %v\n", err)
	return 1
}

func (a *App) failf(format string, args ...any) {
	fmt.Fprintf(a.stderr, "error: "+format+"\n", args...)
}

func (a *App) resolveClient(baseURL string, needAuth bool) (Session, *Client, error) {
	session, err := ResolveSession(baseURL)
	if err != nil {
		return Session{}, nil, err
	}
	if needAuth {
		if err := ensureSession(session); err != nil {
			return Session{}, nil, err
		}
	}
	client := NewClient(session.BaseURL, session.Username, session.Token)
	return session, client, nil
}

func ensureSession(session Session) error {
	if strings.TrimSpace(session.Username) == "" || strings.TrimSpace(session.Token) == "" {
		return errors.New("no local login session, run `of auth login` first")
	}
	return nil
}

func ensureNoExtraArgs(fs *flag.FlagSet) error {
	if fs == nil {
		return nil
	}
	if fs.NArg() == 0 {
		return nil
	}
	return fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
}

func parseUint64CSV(raw string) ([]uint64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	result := make([]uint64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid tag id %q", part)
		}
		result = append(result, value)
	}
	return result, nil
}
