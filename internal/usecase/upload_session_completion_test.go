package usecase

import (
	"context"
	"errors"
	"testing"

	domainnode "omniflow-go/internal/domain/node"
	domainsession "omniflow-go/internal/domain/uploadsession"
	"omniflow-go/internal/storage"
)

type completionObjectStorage struct {
	fakeObjectStorage
	completeErr error
	statInfo    storage.ObjectInfo
	statErr     error
	gotParts    []storage.MultipartUploadPart
}

func (s *completionObjectStorage) CompleteMultipartUpload(
	_ context.Context,
	_ string,
	_ string,
	parts []storage.MultipartUploadPart,
) error {
	s.gotParts = append([]storage.MultipartUploadPart(nil), parts...)
	return s.completeErr
}

func (s *completionObjectStorage) StatObject(
	context.Context,
	string,
) (storage.ObjectInfo, error) {
	return s.statInfo, s.statErr
}

func TestCompleteUploadObjectRecoversAfterLostMultipartResponse(t *testing.T) {
	store := &completionObjectStorage{
		completeErr: errors.New("NoSuchUpload"),
		statInfo:    storage.ObjectInfo{Size: 32},
	}
	session := domainsession.UploadSession{
		ID:            "upload-1",
		Mode:          domainsession.ModeMultipart,
		StorageKey:    "libraries/2/file.bin",
		MinioUploadID: "minio-1",
		FileSize:      32,
	}

	err := completeUploadObject(context.Background(), store, session, []CompletedPart{
		{PartNumber: 2, ETag: "etag-2"},
		{PartNumber: 1, ETag: "etag-1"},
	})
	if err != nil {
		t.Fatalf("expected HEAD recovery to succeed, got %v", err)
	}
	if len(store.gotParts) != 2 || store.gotParts[0].PartNumber != 1 || store.gotParts[1].PartNumber != 2 {
		t.Fatalf("expected sorted parts, got %+v", store.gotParts)
	}
}

func TestCompleteUploadObjectRejectsMismatchedObjectSize(t *testing.T) {
	store := &completionObjectStorage{statInfo: storage.ObjectInfo{Size: 31}}
	session := domainsession.UploadSession{
		Mode:       domainsession.ModeSingle,
		StorageKey: "libraries/2/file.bin",
		FileSize:   32,
	}

	err := completeUploadObject(context.Background(), store, session, nil)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestDecodeUploadCompletionNode(t *testing.T) {
	session := domainsession.UploadSession{
		Status:           domainsession.StatusCommitted,
		CompletedNodeID:  42,
		CompletionResult: `{"id":42,"name":"file","type":"file","libraryId":2}`,
	}
	node, err := decodeUploadCompletionNode(session)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.ID != 42 || node.Type != domainnode.TypeFile {
		t.Fatalf("unexpected node: %+v", node)
	}

	session.CompletedNodeID = 43
	if _, err := decodeUploadCompletionNode(session); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected receipt mismatch conflict, got %v", err)
	}
}

func TestNormalizeUploadClientOperationID(t *testing.T) {
	operationID, err := normalizeUploadClientOperationID(" upload-1 ", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if operationID != "upload:upload-1" {
		t.Fatalf("unexpected legacy operation id: %q", operationID)
	}

	explicit, err := normalizeUploadClientOperationID("upload-1", " operation-1 ")
	if err != nil || explicit != "operation-1" {
		t.Fatalf("unexpected explicit operation id: %q, %v", explicit, err)
	}
}
