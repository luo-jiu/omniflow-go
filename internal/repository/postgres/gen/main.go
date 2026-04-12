package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"unicode"

	"omniflow-go/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

var defaultTableModels = map[string]string{
	"browser_bookmarks":     "BrowserBookmark",
	"browser_file_mappings": "BrowserFileMapping",
	"users":                 "User",
	"libraries":             "Library",
	"nodes":                 "Node",
	"node_files":            "NodeFile",
	"storage_objects":       "StorageObject",
}

func main() {
	configPath := flag.String("config", "configs/config.yaml", "配置文件路径")
	outPath := flag.String("out", "internal/repository/postgres/query", "gen 输出目录")
	modelPath := flag.String("model", "internal/repository/postgres/model", "model 输出目录")
	tableCSV := flag.String("tables", strings.Join(defaultTableList(), ","), "要生成的表名，逗号分隔")
	flag.Parse()

	cfg, err := config.Load(strings.TrimSpace(*configPath))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dsn := strings.TrimSpace(os.Getenv("DATABASE_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(cfg.Database.DSN)
	}
	if dsn == "" {
		log.Fatal("database dsn is empty")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	g := gen.NewGenerator(gen.Config{
		OutPath:           strings.TrimSpace(*outPath),
		ModelPkgPath:      strings.TrimSpace(*modelPath),
		Mode:              gen.WithDefaultQuery | gen.WithQueryInterface,
		FieldNullable:     true,
		FieldWithIndexTag: true,
		FieldWithTypeTag:  true,
	})
	g.UseDB(db)

	tableModels := parseTableModels(*tableCSV)
	if len(tableModels) == 0 {
		log.Fatal("no table specified")
	}

	models := make([]any, 0, len(tableModels))
	for tableName, modelName := range tableModels {
		opts := modelOptions(tableName)
		models = append(models, g.GenerateModelAs(tableName, modelName, opts...))
	}
	g.ApplyBasic(models...)
	g.Execute()

	fmt.Printf("gorm/gen done. out=%s model=%s tables=%s\n",
		strings.TrimSpace(*outPath),
		strings.TrimSpace(*modelPath),
		strings.Join(sortedKeys(tableModels), ","),
	)
}

func parseTableModels(raw string) map[string]string {
	parts := strings.Split(raw, ",")
	out := make(map[string]string, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name == "" {
			continue
		}
		if modelName, ok := defaultTableModels[name]; ok {
			out[name] = modelName
			continue
		}
		out[name] = toUpperCamel(name)
	}
	return out
}

func modelOptions(tableName string) []gen.ModelOpt {
	switch tableName {
	default:
		return nil
	}
}

func defaultTableList() []string {
	return sortedKeys(defaultTableModels)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toUpperCamel(raw string) string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(strings.ToLower(p))
		runes[0] = unicode.ToUpper(runes[0])
		b.WriteString(string(runes))
	}
	return b.String()
}
