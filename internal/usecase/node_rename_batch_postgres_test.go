package usecase

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omniflow-go/internal/actor"
	"omniflow-go/internal/config"
	domainnode "omniflow-go/internal/domain/node"
	"omniflow-go/internal/repository"
	pgmodel "omniflow-go/internal/repository/postgres/model"
)

// TestRenameBatchPostgres 仅显式启用时连接本机 PG，所有写入均进入单连接临时影子表。
func TestRenameBatchPostgres(t *testing.T) {
	if os.Getenv("OMNIFLOW_RENAME_POSTGRES_TEST") != "1" {
		t.Skip("set OMNIFLOW_RENAME_POSTGRES_TEST=1 to run temporary-table PostgreSQL acceptance")
	}
	configPath := os.Getenv("OMNIFLOW_RENAME_TEST_CONFIG")
	if configPath == "" {
		configPath = filepath.Join("..", "..", "configs", "config.yaml")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal("cannot load integration configuration")
	}
	databaseURL, err := url.Parse(cfg.Database.DSN)
	if err != nil || (databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql") ||
		(databaseURL.Hostname() != "127.0.0.1" && databaseURL.Hostname() != "localhost" && databaseURL.Hostname() != "::1") {
		t.Fatal("integration test requires a loopback PostgreSQL URL")
	}
	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot connect to integration PostgreSQL")
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal("cannot access integration connection")
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	outer := db.WithContext(ctx).Begin()
	if outer.Error != nil {
		t.Fatal("cannot begin isolated outer transaction")
	}
	// 即使断言失败或中途取消也回滚；临时表、索引与函数不留下持久对象。
	defer outer.Rollback()
	for _, statement := range []string{
		`SET LOCAL search_path = pg_temp, public`,
		`SET LOCAL statement_timeout = '5s'`,
		`CREATE TEMP TABLE nodes (LIKE public.nodes INCLUDING DEFAULTS INCLUDING CONSTRAINTS) ON COMMIT DROP`,
		`ALTER TABLE pg_temp.nodes ADD PRIMARY KEY (id)`,
		`CREATE UNIQUE INDEX uq_nodes_live_sibling_visible_name ON pg_temp.nodes
		 (library_id, COALESCE(parent_id, 0),
		 (CASE WHEN node_type = 1 AND COALESCE(ext, '') <> '' THEN name || '.' || ext ELSE name END))
		 WHERE deleted_at IS NULL`,
		`CREATE TEMP TABLE node_mutation_receipts
		 (LIKE public.node_mutation_receipts INCLUDING DEFAULTS INCLUDING CONSTRAINTS) ON COMMIT DROP`,
		`ALTER TABLE pg_temp.node_mutation_receipts ADD PRIMARY KEY (actor_id, operation_id)`,
		`CREATE FUNCTION pg_temp.rename_acceptance_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
		 BEGIN NEW.updated_at := TIMESTAMPTZ '2035-01-02T03:04:05.987654Z'; RETURN NEW; END $$`,
		`CREATE TRIGGER rename_acceptance_updated_at BEFORE UPDATE ON pg_temp.nodes
		 FOR EACH ROW EXECUTE FUNCTION pg_temp.rename_acceptance_updated_at()`,
	} {
		if err := outer.Exec(statement).Error; err != nil {
			t.Fatal("cannot prepare isolated PostgreSQL schema; ensure the required migration has run")
		}
	}
	// 普通 repository 使用未限定表名；先证明本连接解析到临时命名空间，再创建任何夹具。
	for _, table := range []string{"nodes", "node_mutation_receipts"} {
		var schema string
		if err := outer.Raw(`SELECT n.nspname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
		 WHERE c.oid=to_regclass(?)`, table).Scan(&schema).Error; err != nil || !strings.HasPrefix(schema, "pg_temp_") {
			t.Fatal("repository tables do not resolve to the temporary namespace")
		}
	}
	principal := actor.Actor{ID: "rename-acceptance", Kind: actor.KindUser}
	u := NewNodeUseCase(repository.NewNodeRepository(outer), repository.NewTransactor(outer), nil, nil)
	initialTime := time.Date(2026, 9, 22, 1, 2, 3, 123456000, time.UTC)
	triggerTime := time.Date(2035, 1, 2, 3, 4, 5, 987654000, time.UTC)
	reset := func(t *testing.T) {
		t.Helper()
		// 两张表始终显式限定 pg_temp；不使用共享 sequence，也不触碰用户节点。
		if err := outer.Exec(`TRUNCATE pg_temp.nodes, pg_temp.node_mutation_receipts`).Error; err != nil {
			t.Fatal("cannot reset temporary fixtures")
		}
		ext, parent := "jpg", int64(800)
		rows := []*pgmodel.Node{
			{ID: 900, LibraryID: 3, ParentID: &parent, NodeType: 1, Name: "first", Ext: &ext, BuiltInType: "DEF", ViewMeta: "{}", CreatedAt: initialTime, UpdatedAt: initialTime},
			{ID: 901, LibraryID: 3, ParentID: &parent, NodeType: 1, Name: "second", Ext: &ext, BuiltInType: "DEF", ViewMeta: "{}", CreatedAt: initialTime, UpdatedAt: initialTime},
			{ID: 902, LibraryID: 3, ParentID: &parent, NodeType: 0, Name: "taken.jpg", BuiltInType: "DEF", ViewMeta: "{}", CreatedAt: initialTime, UpdatedAt: initialTime},
		}
		if err := outer.Create(&rows).Error; err != nil {
			t.Fatal("cannot create explicit-ID temporary fixtures")
		}
	}
	command := func() RenameBatchCommand {
		return RenameBatchCommand{Actor: principal, LibraryID: 3, OperationID: uuid.NewString(), Items: []domainnode.RenameBatchItem{
			{NodeID: 900, Name: "renamed-first", Expected: domainnode.RenameExpected{Name: "first", Ext: "jpg", ParentID: 800, UpdatedAt: initialTime}},
			{NodeID: 901, Name: "renamed-second", Expected: domainnode.RenameExpected{Name: "second", Ext: "jpg", ParentID: 800, UpdatedAt: initialTime}},
		}}
	}
	assertState := func(t *testing.T, names []string, receipts int64) {
		t.Helper()
		var rows []pgmodel.Node
		if err := outer.Where("id IN ?", []int64{900, 901}).Order("id").Find(&rows).Error; err != nil || len(rows) != 2 {
			t.Fatal("cannot read temporary state")
		}
		for i := range rows {
			if rows[i].Name != names[i] {
				t.Fatalf("unexpected temporary node name: %q", rows[i].Name)
			}
		}
		var count int64
		if err := outer.Model(&pgmodel.NodeMutationReceipt{}).Count(&count).Error; err != nil || count != receipts {
			t.Fatalf("unexpected temporary receipt count: %d", count)
		}
	}
	t.Run("success-authoritative-time-replay-status", func(t *testing.T) {
		reset(t)
		cmd := command()
		out, err := u.RenameBatch(ctx, cmd)
		if err != nil || len(out.Items) != 2 || out.State != "committed" || out.Replayed {
			t.Fatalf("unexpected execute result: %+v err=%v", out, err)
		}
		for _, item := range out.Items {
			if !item.Current.UpdatedAt.Equal(triggerTime) || !item.Previous.UpdatedAt.Equal(initialTime) {
				t.Fatalf("receipt did not use authoritative trigger time: %+v", item)
			}
		}
		assertState(t, []string{"renamed-first", "renamed-second"}, 1)
		replay, err := u.RenameBatch(ctx, cmd)
		if err != nil || !replay.Replayed || !replay.Items[0].Current.UpdatedAt.Equal(triggerTime) {
			t.Fatalf("replay failed: %+v err=%v", replay, err)
		}
		status, err := u.RenameBatchStatus(ctx, principal, 3, cmd.OperationID)
		if err != nil || status.State != "committed" || status.Result == nil || status.Result.Replayed {
			t.Fatalf("stored receipt differs from original commit: %+v err=%v", status, err)
		}
		cmd.Items[0].Name = "different"
		if _, err := u.RenameBatch(ctx, cmd); !errors.Is(err, ErrConflict) {
			t.Fatalf("operation hash reuse must fail: %v", err)
		}
		assertState(t, []string{"renamed-first", "renamed-second"}, 1)
	})
	t.Run("dry-run-rolls-back-nodes-and-receipt", func(t *testing.T) {
		reset(t)
		cmd := command()
		cmd.DryRun = true
		out, err := u.RenameBatch(ctx, cmd)
		if err != nil || out.State != "validated" || !out.Items[0].Current.UpdatedAt.Equal(initialTime) {
			t.Fatalf("invalid dry-run result: %+v err=%v", out, err)
		}
		assertState(t, []string{"first", "second"}, 0)
		status, err := u.RenameBatchStatus(ctx, principal, 3, cmd.OperationID)
		if err != nil || status.State != "not_found" || status.Result != nil {
			t.Fatalf("dry-run left a receipt: %+v err=%v", status, err)
		}
	})
	t.Run("second-stale-snapshot-rejects-whole-batch", func(t *testing.T) {
		reset(t)
		cmd := command()
		cmd.Items[1].Expected.UpdatedAt = initialTime.Add(time.Microsecond)
		if _, err := u.RenameBatch(ctx, cmd); !errors.Is(err, ErrConflict) {
			t.Fatalf("stale batch must fail: %v", err)
		}
		assertState(t, []string{"first", "second"}, 0)
	})
	t.Run("visible-name-conflict-with-directory", func(t *testing.T) {
		reset(t)
		cmd := command()
		cmd.Items[1].Name = "taken"
		if _, err := u.RenameBatch(ctx, cmd); !errors.Is(err, ErrConflict) {
			t.Fatalf("file/directory visible-name conflict must fail: %v", err)
		}
		assertState(t, []string{"first", "second"}, 0)
		// 再直接走仓储更新，证明真实唯一索引同样拒绝，而非只靠 usecase 预检查。
		err := repository.NewTransactor(outer).WithinTx(ctx, func(txCtx context.Context) error {
			_, err := repository.NewNodeRepository(outer).WriteLockedRename(txCtx, 3, 900, "taken")
			return err
		})
		if !errors.Is(err, repository.ErrConflict) {
			t.Fatalf("database unique index must reject visible collision: %v", err)
		}
		assertState(t, []string{"first", "second"}, 0)
	})
	if err := outer.Rollback().Error; err != nil {
		t.Fatal("isolated outer transaction rollback failed")
	}
}
