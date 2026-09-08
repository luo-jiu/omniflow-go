package repository

import (
	"encoding/json"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	domainnode "omniflow-go/internal/domain/node"
	pgmodel "omniflow-go/internal/repository/postgres/model"
	"strings"
	"testing"
)

func TestMetadataQueryUsesBoundedScopedSQL(t *testing.T) {
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=localhost user=fixture dbname=fixture sslmode=disable"}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return metadataQuery(tx, domainnode.MetadataQuery{LibraryID: 3, Mode: "children", ParentID: 10, Keyword: "100%_", NodeType: domainnode.TypeDirectory}, 10, 20, 21).Find(&[]pgmodel.Node{})
	})
	for _, part := range []string{"library_id = 3", "id > 20", "parent_id = 10", "node_type = 0", "deleted_at", "ORDER BY id ASC", "LIMIT 21", "100\\%\\_"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("SQL missing %q: %s", part, sql)
		}
	}
	if strings.Count(sqlMetadataSubtreeIDs, "?") != 3 || strings.Count(sqlMetadataPaths, "?") != 5 {
		t.Fatal("recursive query parameter shape changed")
	}
}

func TestMetadataProjectionOmitsStorageSecretsAndBuildsFilePath(t *testing.T) {
	node := domainnode.Node{ID: 8, LibraryID: 3, ParentID: 10, Name: "video", Ext: "mp4", Type: domainnode.TypeFile, StorageKey: "private-key", StorageEndpoint: "private-endpoint", StorageProvider: "mac"}
	entry := metadataEntry(node, "/Videos/video")
	data, _ := json.Marshal(entry)
	if entry.Path != "/Videos/video.mp4" || strings.Contains(string(data), "private") {
		t.Fatalf("unsafe or invalid metadata: %s", data)
	}
}
