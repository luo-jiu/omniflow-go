package usecase

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	"omniflow-go/internal/actor"
	domainlibrary "omniflow-go/internal/domain/library"
	domainnode "omniflow-go/internal/domain/node"
	domainuser "omniflow-go/internal/domain/user"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

var (
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource conflict")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnsupportedStorage = errors.New("unsupported object storage implementation")
)

const (
	userStatusActive   = 1
	userStatusDisabled = 2
	userStatusPending  = 3

	nodeTypeDirectory = 0
	nodeTypeFile      = 1
)

var sharedRedisClient atomic.Pointer[redis.Client]

func setSharedRedisClient(client *redis.Client) {
	if client != nil {
		sharedRedisClient.Store(client)
	}
}

func getSharedRedisClient() *redis.Client {
	return sharedRedisClient.Load()
}

func dbFromRepository(repo any) (*gorm.DB, error) {
	if repo == nil {
		return nil, fmt.Errorf("%w: repository is nil", ErrInvalidArgument)
	}

	value := reflect.ValueOf(repo)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return nil, fmt.Errorf("%w: repository must be a non-nil pointer", ErrInvalidArgument)
	}

	elem := value.Elem()
	field := elem.FieldByName("db")
	if !field.IsValid() {
		return nil, fmt.Errorf("%w: repository has no db field", ErrInvalidArgument)
	}
	if field.IsNil() {
		return nil, fmt.Errorf("%w: repository db is nil", ErrInvalidArgument)
	}
	if field.CanInterface() {
		db, ok := field.Interface().(*gorm.DB)
		if !ok || db == nil {
			return nil, fmt.Errorf("%w: repository db has unexpected type", ErrInvalidArgument)
		}
		return db, nil
	}
	if !field.CanAddr() {
		return nil, fmt.Errorf("%w: repository db is not addressable", ErrInvalidArgument)
	}

	// repository.db is an unexported field in another package.
	dbPtr := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	db, ok := dbPtr.Interface().(*gorm.DB)
	if !ok || db == nil {
		return nil, fmt.Errorf("%w: repository db has unexpected type", ErrInvalidArgument)
	}
	return db, nil
}

func actorIDToUint64(principal actor.Actor) (uint64, error) {
	id := strings.TrimSpace(principal.ID)
	if id == "" {
		return 0, fmt.Errorf("%w: actor id is required", ErrInvalidArgument)
	}
	parsed, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: actor id must be numeric", ErrInvalidArgument)
	}
	return parsed, nil
}

func nodeTypeToCode(t domainnode.Type) int {
	if t == domainnode.TypeFile {
		return nodeTypeFile
	}
	return nodeTypeDirectory
}

func nodeTypeFromCode(code int) domainnode.Type {
	if code == nodeTypeFile {
		return domainnode.TypeFile
	}
	return domainnode.TypeDirectory
}

func userStatusFromCode(code int) domainuser.Status {
	switch code {
	case userStatusActive:
		return domainuser.StatusActive
	case userStatusDisabled:
		return domainuser.StatusDisabled
	default:
		return domainuser.StatusPending
	}
}

type userRecord struct {
	ID           uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Username     string         `gorm:"column:username"`
	Nickname     string         `gorm:"column:nickname"`
	PasswordHash string         `gorm:"column:password_hash"`
	Phone        string         `gorm:"column:phone"`
	Email        string         `gorm:"column:email"`
	ExtJSON      string         `gorm:"column:ext"`
	Status       int            `gorm:"column:status"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (userRecord) TableName() string {
	return "users"
}

func (r userRecord) toDomain() domainuser.User {
	nickname := strings.TrimSpace(r.Nickname)
	if nickname == "" {
		nickname = r.Username
	}

	return domainuser.User{
		ID:       r.ID,
		Username: r.Username,
		Nickname: nickname,
		Phone:    r.Phone,
		Email:    r.Email,
		Ext:      r.ExtJSON,
		Status:   userStatusFromCode(r.Status),
	}
}

type libraryRecord struct {
	ID        uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    uint64         `gorm:"column:user_id"`
	Name      string         `gorm:"column:name"`
	Starred   bool           `gorm:"column:starred"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (libraryRecord) TableName() string {
	return "libraries"
}

func (r libraryRecord) toDomain() domainlibrary.Library {
	return domainlibrary.Library{
		ID:     r.ID,
		UserID: r.UserID,
		Name:   r.Name,
	}
}

type nodeRecord struct {
	ID         uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	Name       string         `gorm:"column:name"`
	Ext        *string        `gorm:"column:ext"`
	MIMEType   string         `gorm:"column:mime_type;->"`
	FileSize   int64          `gorm:"column:file_size;->"`
	StorageKey string         `gorm:"column:storage_key;->"`
	BuiltIn    string         `gorm:"column:built_in_type"`
	NodeType   int            `gorm:"column:node_type"`
	Archive    bool           `gorm:"column:archive_mode"`
	SortOrder  int            `gorm:"column:sort_order"`
	ParentID   *uint64        `gorm:"column:parent_id"`
	LibraryID  uint64         `gorm:"column:library_id"`
	DeletedAt  gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (nodeRecord) TableName() string {
	return "nodes"
}

func (r nodeRecord) toDomain() domainnode.Node {
	ext := ""
	if r.Ext != nil {
		ext = *r.Ext
	}

	return domainnode.Node{
		ID:         r.ID,
		Name:       r.Name,
		Type:       nodeTypeFromCode(r.NodeType),
		ParentID:   parentIDValue(r.ParentID),
		LibraryID:  r.LibraryID,
		Ext:        ext,
		MIMEType:   r.MIMEType,
		FileSize:   r.FileSize,
		StorageKey: r.StorageKey,
	}
}

type storageObjectRecord struct {
	ID            uint64         `gorm:"column:id;primaryKey;autoIncrement"`
	LibraryID     uint64         `gorm:"column:library_id"`
	Provider      string         `gorm:"column:provider"`
	Bucket        string         `gorm:"column:bucket"`
	ObjectKey     string         `gorm:"column:object_key"`
	ContentLength int64          `gorm:"column:content_length"`
	ContentType   string         `gorm:"column:content_type"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (storageObjectRecord) TableName() string {
	return "storage_objects"
}

type nodeFileRecord struct {
	FileID          uint64 `gorm:"column:file_id;primaryKey"`
	LibraryID       uint64 `gorm:"column:library_id"`
	StorageObjectID uint64 `gorm:"column:storage_object_id"`
	MIMEType        string `gorm:"column:mime_type"`
	FileSize        int64  `gorm:"column:file_size"`
}

func (nodeFileRecord) TableName() string {
	return "node_files"
}
