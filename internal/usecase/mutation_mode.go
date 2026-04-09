package usecase

import "errors"

const (
	executionModeExecute = "execute"
	executionModeDryRun  = "dry-run"
)

var errUsecaseDryRunRollback = errors.New("usecase dry-run rollback")

func resolveMutationMode(dryRun bool) string {
	if dryRun {
		return executionModeDryRun
	}
	return executionModeExecute
}
