package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

// presignBatchConcurrency 与前端 upload-direct.ts 保持一致：单批 4 个 part 并发签名+PUT。
const presignBatchConcurrency = 4

func (a *App) runUploadFile(args []string) error {
	fs := a.newFlagSet("upload file")

	var (
		baseURL         string
		libraryID       uint64
		parentID        uint64
		filePath        string
		storageProvider string
		conflictPolicy  string
		contentType     string
		jsonOut         bool
	)
	fs.StringVar(&baseURL, "base-url", "", "API base url")
	fs.Uint64Var(&libraryID, "library-id", 0, "library id (required)")
	fs.Uint64Var(&parentID, "parent-id", 0, "parent node id (optional, default root)")
	fs.StringVar(&filePath, "file", "", "local file path to upload (required)")
	fs.StringVar(&storageProvider, "storage-provider", "", "storage provider id (optional)")
	fs.StringVar(&conflictPolicy, "conflict-policy", "", "name conflict strategy: error / auto_rename / replace")
	fs.StringVar(&contentType, "content-type", "", "override content type (optional)")
	fs.BoolVar(&jsonOut, "json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := ensureNoExtraArgs(fs); err != nil {
		return err
	}
	if libraryID == 0 {
		return errors.New("`--library-id` is required")
	}

	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return errors.New("`--file` is required")
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat upload file: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("`--file` must be a regular file: %s", filePath)
	}
	if stat.Size() <= 0 {
		return fmt.Errorf("upload file is empty: %s", filePath)
	}

	_, client, err := a.resolveClient(baseURL, true)
	if err != nil {
		return err
	}

	// 捕 SIGINT/SIGTERM：尽力调 UploadAbort 收尾 MinIO + session。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	initRes, err := client.UploadInit(ctx, UploadInitRequest{
		LibraryID:       libraryID,
		ParentID:        parentID,
		FileName:        filepath.Base(filePath),
		FileSize:        stat.Size(),
		ContentType:     strings.TrimSpace(contentType),
		StorageProvider: strings.TrimSpace(storageProvider),
	})
	if err != nil {
		return fmt.Errorf("upload init: %w", err)
	}

	uploadID := initRes.UploadID

	// 安装 abort：信号或后续错误都会调用。abortOnce 防多次触发。
	var abortOnce sync.Once
	doAbort := func() {
		abortOnce.Do(func() {
			abortCtx, abortCancel := context.WithCancel(context.Background())
			defer abortCancel()
			if abortErr := client.UploadAbort(abortCtx, uploadID); abortErr != nil {
				fmt.Fprintf(a.stderr, "warning: abort upload session failed: %v\n", abortErr)
			}
		})
	}

	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(a.stderr, "received signal, aborting upload session...")
			cancel()
			doAbort()
		case <-ctx.Done():
		}
	}()

	// 进度反馈：JSON 模式下静默（stdout 留给结构化结果）；否则每完成一个 part 在 stderr 用 \r 原地刷新。
	var progress *uploadProgressPrinter
	if !jsonOut {
		progress = newUploadProgressPrinter(a.stderr, stat.Size())
	}

	collectedEtags, err := a.uploadAllParts(ctx, client, filePath, initRes, contentType, progress)
	if err != nil {
		if progress != nil {
			progress.finish(false)
		}
		doAbort()
		return err
	}
	if progress != nil {
		progress.finish(true)
	}

	completeReq := UploadCompleteRequest{
		UploadID:       uploadID,
		ConflictPolicy: strings.TrimSpace(conflictPolicy),
	}
	if initRes.Mode != "single" {
		completeReq.Parts = make([]UploadCompletedPart, 0, len(collectedEtags))
		for partNumber := 1; partNumber <= initRes.TotalParts; partNumber++ {
			completeReq.Parts = append(completeReq.Parts, UploadCompletedPart{
				PartNumber: partNumber,
				ETag:       collectedEtags[partNumber],
			})
		}
	}

	node, err := client.UploadComplete(ctx, completeReq)
	if err != nil {
		doAbort()
		return fmt.Errorf("upload complete: %w", err)
	}

	if jsonOut {
		return a.printJSON(node)
	}
	a.printf(
		"uploaded: id=%d name=%s library=%d parent=%d size=%d mode=%s parts=%d\n",
		node.ID, node.Name, node.LibraryID, node.ParentID, node.FileSize, initRes.Mode, initRes.TotalParts,
	)
	return nil
}

// uploadAllParts 按 PRESIGN_BATCH_CONCURRENCY 批量签名并发 PUT，所有 part 完成后返回 partNumber→etag 映射。
// progress 可空（JSON 模式下不打印）。
func (a *App) uploadAllParts(
	ctx context.Context,
	client *Client,
	filePath string,
	initRes UploadInitResult,
	contentType string,
	progress *uploadProgressPrinter,
) (map[int]string, error) {
	totalParts := initRes.TotalParts
	if totalParts <= 0 {
		totalParts = 1
	}

	collected := make(map[int]string, totalParts)
	var collectedMu sync.Mutex

	for cursor := 0; cursor < totalParts; cursor += presignBatchConcurrency {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		end := min(cursor+presignBatchConcurrency, totalParts)

		batchNumbers := make([]int, 0, end-cursor)
		for partNumber := cursor + 1; partNumber <= end; partNumber++ {
			batchNumbers = append(batchNumbers, partNumber)
		}

		signRes, err := client.UploadSignParts(ctx, UploadSignPartsRequest{
			UploadID:    initRes.UploadID,
			PartNumbers: batchNumbers,
		})
		if err != nil {
			return nil, fmt.Errorf("sign upload parts: %w", err)
		}

		signedURL := make(map[int]string, len(signRes.Parts))
		for _, part := range signRes.Parts {
			signedURL[part.PartNumber] = part.URL
		}

		var (
			wg      sync.WaitGroup
			firstMu sync.Mutex
			firstEr error
		)
		for _, partNumber := range batchNumbers {
			presignedURL, ok := signedURL[partNumber]
			if !ok {
				return nil, fmt.Errorf("server did not return presigned url for part %d", partNumber)
			}

			byteOffset := int64(partNumber-1) * initRes.PartSize
			byteLength := initRes.PartSize

			wg.Go(func() {
				etag, n, err := putSinglePart(ctx, client, presignedURL, filePath, byteOffset, byteLength, contentType, partNumber, initRes)
				if err != nil {
					firstMu.Lock()
					if firstEr == nil {
						firstEr = err
					}
					firstMu.Unlock()
					return
				}
				collectedMu.Lock()
				collected[partNumber] = etag
				collectedMu.Unlock()
				if progress != nil {
					progress.add(n)
				}
			})
		}
		wg.Wait()

		if firstEr != nil {
			return nil, firstEr
		}
	}

	return collected, nil
}

// putSinglePart 打开文件 + Seek 到 byteOffset，PUT 指定长度的字节区间。
// 返回 (etag, 实际 PUT 的字节数, error)。字节数用于上层进度合并。
func putSinglePart(
	ctx context.Context,
	client *Client,
	presignedURL string,
	filePath string,
	byteOffset int64,
	byteLength int64,
	contentType string,
	partNumber int,
	initRes UploadInitResult,
) (string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", 0, fmt.Errorf("open file for part %d: %w", partNumber, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", 0, fmt.Errorf("stat file for part %d: %w", partNumber, err)
	}

	// 末尾 part：剩余字节可能小于 PartSize；single 模式整个文件就是一个 part。
	effectiveLength := byteLength
	if initRes.Mode == "single" {
		effectiveLength = stat.Size()
		byteOffset = 0
	} else if byteOffset+effectiveLength > stat.Size() {
		effectiveLength = stat.Size() - byteOffset
	}
	if effectiveLength <= 0 {
		return "", 0, fmt.Errorf("computed part %d length <= 0 (offset=%d filesize=%d)", partNumber, byteOffset, stat.Size())
	}

	if _, err := file.Seek(byteOffset, 0); err != nil {
		return "", 0, fmt.Errorf("seek file for part %d: %w", partNumber, err)
	}

	limited := &boundedReader{r: file, remaining: effectiveLength}
	etag, err := client.PresignedPut(ctx, presignedURL, limited, effectiveLength, contentType)
	if err != nil {
		return "", 0, fmt.Errorf("put part %d: %w", partNumber, err)
	}
	if etag == "" {
		return "", 0, fmt.Errorf("server did not return ETag for part %d", partNumber)
	}
	return etag, effectiveLength, nil
}

// boundedReader 限制读取上限，防止 http.Client 把整个文件 read 完（PUT 单 part 时只想读 byteLength）。
// 用 io.LimitReader 也能做到，但 LimitReader.N 公开可改，自己实现更清晰。
type boundedReader struct {
	r         io.Reader
	remaining int64
}

func (b *boundedReader) Read(p []byte) (int, error) {
	if b.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > b.remaining {
		p = p[:b.remaining]
	}
	n, err := b.r.Read(p)
	b.remaining -= int64(n)
	return n, err
}
