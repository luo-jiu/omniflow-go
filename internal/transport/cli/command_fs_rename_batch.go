package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	domainnode "omniflow-go/internal/domain/node"
)

// RenameBatchRequest 对应服务端条件批量改名契约。
type RenameBatchRequest struct {
	LibraryID   uint64                       `json:"libraryId"`
	OperationID string                       `json:"operationId"`
	Items       []domainnode.RenameBatchItem `json:"items"`
}

// RenameBatch 发出一次原子改名请求，不自动重试不明结果。
func (c *Client) RenameBatch(ctx context.Context, req RenameBatchRequest, dryRun bool) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, http.MethodPost, "/api/v1/nodes/rename/batch/conditional", withDryRunQuery(nil, dryRun), req, true, &out)
	return out, err
}

// RenameBatchStatus 读取当前身份的回执，未找到不代表没有在途请求。
func (c *Client) RenameBatchStatus(ctx context.Context, libraryID uint64, operationID string) (map[string]any, error) {
	var out map[string]any
	err := c.doJSON(ctx, http.MethodGet, "/api/v1/nodes/rename/batch/status", url.Values{
		"libraryId": {strconv.FormatUint(libraryID, 10)}, "operationId": {operationID},
	}, nil, true, &out)
	return out, err
}

func (a *App) runFSRenameBatch(args []string) error  { return a.runRenameBatchCommand(args, true) }
func (a *App) runFSRenameStatus(args []string) error { return a.runRenameBatchCommand(args, false) }

func (a *App) runRenameBatchCommand(args []string, write bool) error {
	fs := a.newFlagSet("fs rename-batch")
	var req RenameBatchRequest
	var baseURL, filePath string
	var dryRun, jsonOut bool
	fs.Uint64Var(&req.LibraryID, "library-id", 0, "library ID (required)")
	fs.StringVar(&req.OperationID, "operation-id", "", "canonical UUID operation ID (required)")
	fs.StringVar(&baseURL, "base-url", "", "API base URL")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if write {
		fs.StringVar(&filePath, "file", "", "JSON file containing the items array (required)")
		fs.BoolVar(&dryRun, "dry-run", false, "validate without committing")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if req.LibraryID == 0 || req.OperationID == "" || (write && filePath == "") {
		return errors.New("--library-id, --operation-id and (for writes) --file are required")
	}
	if write {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, (128<<10)+1))
		if err != nil {
			return err
		}
		if len(data) > 128<<10 {
			return errors.New("rename proposal exceeds 128 KiB")
		}
		// 保留原始条目 JSON 给 HTTP 端点校验缺失字段，不能把缺失 ext 填成空字符串。
		var rawItems []json.RawMessage
		if err := json.Unmarshal(data, &rawItems); err != nil || len(rawItems) == 0 || len(rawItems) > 50 {
			return errors.New("--file must contain a JSON array of 1 to 50 rename items")
		}
		_, client, err := a.resolveClient(baseURL, true)
		if err != nil {
			return err
		}
		var out map[string]any
		err = client.doJSON(context.Background(), http.MethodPost, "/api/v1/nodes/rename/batch/conditional",
			withDryRunQuery(nil, dryRun), map[string]any{"libraryId": req.LibraryID, "operationId": req.OperationID, "items": rawItems}, true, &out)
		if err != nil {
			return fmt.Errorf("%w (operationId=%s; query fs rename-status before retrying)", err, req.OperationID)
		}
		return a.printRenameBatch(out, req.OperationID, dryRun, jsonOut)
	}
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	out, err := client.RenameBatchStatus(context.Background(), req.LibraryID, req.OperationID)
	if err != nil {
		return err
	}
	return a.printRenameBatch(out, req.OperationID, false, jsonOut)
}

func (a *App) printRenameBatch(out map[string]any, operationID string, dryRun, jsonOut bool) error {
	if jsonOut {
		return a.printJSON(out)
	}
	if dryRun {
		if result, ok := out["result"].(map[string]any); ok {
			out = result
		}
	}
	a.printf("rename operation=%s state=%v dry_run=%t\n", operationID, out["state"], dryRun)
	return nil
}
