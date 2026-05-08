package cli

import (
	"context"
	"errors"
	"strings"
)

func (a *App) runStorageMigrate(args []string) error {
	fs := a.newFlagSet("storage migrate")

	var (
		baseURL        string
		libraryID      int64
		nodeID         int64
		targetProvider string
		dryRun         bool
		jsonOut        bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Int64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Int64Var(&nodeID, "node-id", 0, "root node id (required)")
	fs.StringVar(&targetProvider, "target-provider", "", "target provider alias (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	if libraryID <= 0 {
		return errors.New("`--library-id` is required and must be > 0")
	}
	if nodeID <= 0 {
		return errors.New("`--node-id` is required and must be > 0")
	}
	targetProvider = strings.TrimSpace(targetProvider)
	if targetProvider == "" {
		return errors.New("`--target-provider` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	result, err := client.MigrationEnqueue(context.Background(), MigrationEnqueueRequest{
		LibraryID:      libraryID,
		RootNodeID:     nodeID,
		TargetProvider: targetProvider,
	}, dryRun)
	if err != nil {
		return err
	}

	if jsonOut {
		return a.printJSON(result)
	}
	if dryRun {
		a.printf("dry-run: would enqueue %d objects (%d bytes) to provider %s bucket %s\n",
			result.PlannedObjects, result.PlannedBytes, result.TargetProvider, result.TargetBucket)
		return nil
	}
	if result.Task == nil {
		a.println("migration enqueued (no task returned)")
		return nil
	}
	a.printf("migration task enqueued: id=%s status=%s plannedObjects=%d plannedBytes=%d targetProvider=%s\n",
		result.Task.ID, result.Task.Status, result.PlannedObjects, result.PlannedBytes, result.TargetProvider)
	return nil
}

func (a *App) runMigrationList(args []string) error {
	fs := a.newFlagSet("migration ls")

	var (
		baseURL    string
		libraryID  int64
		statusList string
		limit      int
		jsonOut    bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Int64Var(&libraryID, "library-id", 0, "library id filter")
	fs.StringVar(&statusList, "status", "", "comma-separated status filter (e.g. running,pending)")
	fs.IntVar(&limit, "limit", 0, "max result size")
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

	statuses := splitCSV(statusList)
	result, err := client.MigrationListTasks(context.Background(), libraryID, statuses, limit)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(result)
	}
	if len(result.Tasks) == 0 {
		a.println("no migration tasks")
		return nil
	}
	for _, t := range result.Tasks {
		a.printf("%s  status=%-10s lib=%d node=%d target=%s progress=%d/%d bytes=%d/%d\n",
			t.ID, t.Status, t.LibraryID, t.RootNodeID, t.TargetProvider,
			t.CompletedObjects, t.TotalObjects, t.TransferredBytes, t.TotalBytes)
	}
	return nil
}

func (a *App) runMigrationStatus(args []string) error {
	fs := a.newFlagSet("migration status")

	var (
		baseURL   string
		taskID    string
		showItems bool
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&taskID, "task-id", "", "migration task id (required)")
	fs.BoolVar(&showItems, "items", false, "include task items")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("`--task-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	taskResp, err := client.MigrationGetTask(context.Background(), taskID)
	if err != nil {
		return err
	}

	var items []MigrationTaskItem
	if showItems {
		itemResp, err := client.MigrationListTaskItems(context.Background(), taskID)
		if err != nil {
			return err
		}
		items = itemResp.Items
	}

	if jsonOut {
		payload := map[string]any{"task": taskResp.Task}
		if showItems {
			payload["items"] = items
		}
		return a.printJSON(payload)
	}

	t := taskResp.Task
	a.printf("task %s\n  status:   %s\n  library:  %d\n  rootNode: %d\n  target:   %s\n",
		t.ID, t.Status, t.LibraryID, t.RootNodeID, t.TargetProvider)
	a.printf("  objects:  %d/%d (failed=%d skipped=%d)\n",
		t.CompletedObjects, t.TotalObjects, t.FailedObjects, t.SkippedObjects)
	a.printf("  bytes:    %d/%d\n", t.TransferredBytes, t.TotalBytes)
	if t.CurrentObjectKey != "" {
		a.printf("  current:  %s\n", t.CurrentObjectKey)
	}
	if t.ErrorMessage != "" {
		a.printf("  error:    %s\n", t.ErrorMessage)
	}
	if showItems {
		a.println("\nitems:")
		for _, it := range items {
			a.printf("  #%d %-9s %d bytes  %s/%s -> %s  %s\n",
				it.ID, it.Status, it.FileSize, it.SourceProvider, it.SourceBucket, it.TargetKey, it.ErrorMessage)
		}
	}
	return nil
}

func (a *App) runMigrationCancel(args []string) error {
	fs := a.newFlagSet("migration cancel")

	var (
		baseURL string
		taskID  string
		dryRun  bool
		jsonOut bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.StringVar(&taskID, "task-id", "", "migration task id (required)")
	fs.BoolVar(&dryRun, "dry-run", false, "preview only, do not commit")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return errors.New("`--task-id` is required")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	if err := client.MigrationCancelTask(context.Background(), taskID, dryRun); err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(map[string]any{"taskId": taskID, "dryRun": dryRun, "canceled": !dryRun})
	}
	if dryRun {
		a.printf("dry-run: task %s is cancellable\n", taskID)
		return nil
	}
	a.printf("task %s canceled\n", taskID)
	return nil
}

func (a *App) runStorageDistribution(args []string) error {
	fs := a.newFlagSet("storage distribution")

	var (
		baseURL   string
		libraryID int64
		nodeID    int64
		jsonOut   bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Int64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Int64Var(&nodeID, "node-id", 0, "root node id (required)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}

	if libraryID <= 0 {
		return errors.New("`--library-id` is required and must be > 0")
	}
	if nodeID <= 0 {
		return errors.New("`--node-id` is required and must be > 0")
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	dist, err := client.StorageDistribution(context.Background(), libraryID, nodeID)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(dist)
	}
	if len(dist.ByProvider) == 0 {
		a.println("no storage objects under this node")
		return nil
	}
	for _, e := range dist.ByProvider {
		a.printf("%-20s files=%d bytes=%d\n", e.Provider, e.FileCount, e.TotalBytes)
	}
	return nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
