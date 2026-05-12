package cli

import (
	"context"
	"errors"
)

func (a *App) runResourceMonitorSample(args []string) error {
	fs := a.newFlagSet("resource-monitor sample")

	var (
		baseURL   string
		libraryID int64
		dryRun    bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Int64Var(&libraryID, "library-id", 0, "optional library scope")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if libraryID < 0 {
		return errors.New("`--library-id` must be >= 0")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	sample, err := client.CaptureResourceMonitorSample(context.Background(), libraryID, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(sample)
	}
	if dryRun {
		a.printf(
			"dry-run: would capture resource monitor sample scope=%s library=%d bytes=%d probes=%d/%d\n",
			sample.Scope,
			sample.LibraryID,
			sample.PhysicalBytes,
			sample.ProbeOK,
			sample.ProbeTotal,
		)
		return nil
	}
	a.printf(
		"resource monitor sample captured: id=%d scope=%s library=%d bytes=%d probes=%d/%d\n",
		sample.ID,
		sample.Scope,
		sample.LibraryID,
		sample.PhysicalBytes,
		sample.ProbeOK,
		sample.ProbeTotal,
	)
	return nil
}
