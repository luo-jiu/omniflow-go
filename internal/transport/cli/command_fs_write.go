package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

func (a *App) runFSWrite(args []string) error {
	fs := a.newFlagSet("fs write")
	var baseURL, file, expected string
	var libraryID, nodeID uint64
	var dryRun, jsonOut bool
	fs.StringVar(&baseURL, "base-url", "", "API base URL")
	fs.StringVar(&file, "file", "", "UTF-8 content file")
	fs.StringVar(&expected, "expected-storage-key", "", "optional conditional write token from node metadata")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id")
	fs.Uint64Var(&nodeID, "node-id", 0, "existing file node id")
	fs.BoolVar(&dryRun, "dry-run", false, "validate without writing")
	fs.BoolVar(&jsonOut, "json", false, "JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if libraryID == 0 || nodeID == 0 || file == "" {
		return errors.New("--library-id, --node-id and --file are required")
	}
	source, err := os.Open(file)
	if err != nil {
		return err
	}
	defer source.Close()
	content, err := io.ReadAll(io.LimitReader(source, 8*1024*1024+1))
	if err != nil {
		return err
	}
	if len(content) > 8*1024*1024 || !utf8.Valid(content) {
		return errors.New("content must be UTF-8 and at most 8 MiB")
	}
	var condition *string
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "expected-storage-key" {
			condition = &expected
		}
	})
	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}
	node, err := client.WriteFileContent(context.Background(), nodeID, WriteFileContentRequest{LibraryID: libraryID, Content: string(content), ExpectedStorageKey: condition}, dryRun)
	if err != nil {
		return err
	}
	if jsonOut {
		return a.printJSON(map[string]any{"node": node, "dryRun": dryRun})
	}
	a.printf("file content validated%s: node=%d\n", map[bool]string{true: " (dry-run)", false: " and saved"}[dryRun], nodeID)
	return nil
}

// WriteFileContentRequest 文本写入及可选的存储版本条件。
type WriteFileContentRequest struct {
	LibraryID          uint64  `json:"libraryId"`
	Content            string  `json:"content"`
	ExpectedStorageKey *string `json:"expectedStorageKey,omitempty"`
}

// WriteFileContent 通过 HTTP 保存文本；提供条件时仅调用条件写入端点。
func (c *Client) WriteFileContent(ctx context.Context, nodeID uint64, input WriteFileContentRequest, dryRun bool) (Node, error) {
	endpoint := fmt.Sprintf("/api/v1/nodes/%d/content", nodeID)
	if input.ExpectedStorageKey != nil {
		endpoint += "/conditional"
	}
	var node Node
	err := c.doJSON(ctx, "PUT", endpoint, withDryRunQuery(nil, dryRun), input, true, &node)
	return node, err
}
