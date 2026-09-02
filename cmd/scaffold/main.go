package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/mod/module"
)

type options struct {
	module      string
	name        string
	database    string
	telemetry   string
	redis       bool
	mqtt        bool
	redisStream bool
	frontend    string
	dryRun      bool
}

func main() {
	var opts options
	flag.StringVar(&opts.module, "module", "", "Go module path")
	flag.StringVar(&opts.name, "name", "", "service name")
	flag.StringVar(&opts.database, "database", "postgres", "postgres, mysql, or none")
	flag.StringVar(&opts.telemetry, "telemetry", "none", "clickhouse, tdengine, or none")
	flag.BoolVar(&opts.redis, "redis", true, "include Redis")
	flag.BoolVar(&opts.mqtt, "mqtt", false, "include MQTT client")
	flag.BoolVar(&opts.redisStream, "redis-stream", false, "include Redis Stream consumer")
	flag.StringVar(&opts.frontend, "frontend", "none", "embed or none")
	flag.BoolVar(&opts.dryRun, "dry-run", false, "show changes without writing")
	flag.Parse()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if err := validate(&opts); err != nil {
		return err
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || !strings.HasPrefix(string(goMod), "module go-web-template\n") {
		return fmt.Errorf("run scaffold once from an uninitialized go-web-template root")
	}
	deletions := deletionList(opts)
	fmt.Printf("module=%s name=%s database=%s telemetry=%s redis=%t mqtt=%t redis-stream=%t frontend=%s\n", opts.module, opts.name, opts.database, opts.telemetry, opts.redis, opts.mqtt, opts.redisStream, opts.frontend)
	for _, path := range deletions {
		fmt.Printf("delete %s\n", path)
	}
	if opts.dryRun {
		return nil
	}
	for _, path := range deletions {
		if err := os.RemoveAll(filepath.Join(root, path)); err != nil {
			return fmt.Errorf("delete %s: %w", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "internal/storage/storage.go"), []byte(renderStorage(opts)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "internal/integration/integration.go"), []byte(renderIntegrations(opts)), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "compose.yaml"), []byte(renderCompose(opts)), 0o644); err != nil {
		return err
	}
	if err := filterExamples(root, opts); err != nil {
		return err
	}
	if err := adaptTests(root, opts.database); err != nil {
		return err
	}
	if err := adaptRouter(root, opts.frontend); err != nil {
		return err
	}
	if err := adaptReadme(root, opts); err != nil {
		return err
	}
	if err := replaceModule(root, opts.module, opts.name); err != nil {
		return err
	}
	for _, command := range [][]string{{"gofmt", "-w", "cmd", "internal", "migrations"}, {"go", "mod", "tidy"}, {"go", "test", "./..."}} {
		cmd := exec.Command(command[0], command[1:]...)
		cmd.Dir = root
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("run %s: %w", strings.Join(command, " "), err)
		}
	}
	return nil
}

func adaptTests(root, database string) error {
	if database != "mysql" {
		return nil
	}
	for _, filename := range []string{"internal/note/repository_test.go", "internal/httpapi/router_test.go"} {
		path := filepath.Join(root, filename)
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(contents), "gorm.io/driver/postgres", "gorm.io/driver/mysql")
		updated = strings.ReplaceAll(updated, "postgres.New(postgres.Config{Conn: sqlDB})", "mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true})")
		updated = strings.ReplaceAll(updated, "`SELECT count(*) FROM \"notes\"`", "\"SELECT count(*) FROM `notes`\"")
		updated = strings.ReplaceAll(updated, "`SELECT * FROM \"notes\" ORDER BY id LIMIT $1 OFFSET $2`", "\"SELECT * FROM `notes` ORDER BY id LIMIT ? OFFSET ?\"")
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func adaptReadme(root string, opts options) error {
	path := filepath.Join(root, "README.md")
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := string(contents)
	introEnd := strings.Index(updated, "\n\n## 创建项目")
	if introEnd > 0 {
		introStart := strings.Index(updated, "\n\n")
		features := []string{"Gin", "GORM"}
		if opts.database != "none" {
			features = append(features, map[string]string{"mysql": "MySQL", "postgres": "PostgreSQL"}[opts.database])
		}
		if opts.telemetry != "none" {
			features = append(features, map[string]string{"clickhouse": "ClickHouse", "tdengine": "TDengine"}[opts.telemetry])
		}
		if opts.redis {
			features = append(features, "Redis")
		}
		if opts.mqtt {
			features = append(features, "MQTT")
		}
		if opts.redisStream {
			features = append(features, "Redis Stream")
		}
		if opts.frontend == "embed" {
			features = append(features, "嵌入前端")
		}
		updated = updated[:introStart+2] + "可扩展的 Go Web 服务，已选择 " + strings.Join(features, "、") + "。未选择的存储实现不会进入依赖和二进制。" + updated[introEnd:]
	}
	if start, end := strings.Index(updated, "## 创建项目\n"), strings.Index(updated, "## 运行\n"); start >= 0 && end > start {
		selected := []string{"gin"}
		if opts.database != "none" {
			selected = append(selected, opts.database)
		}
		if opts.telemetry != "none" {
			selected = append(selected, opts.telemetry)
		}
		if opts.redis {
			selected = append(selected, "redis")
		}
		if opts.mqtt {
			selected = append(selected, "mqtt")
		}
		if opts.redisStream {
			selected = append(selected, "redis-stream")
		}
		if opts.frontend == "embed" {
			selected = append(selected, "frontend-embed")
		}
		updated = updated[:start] + "## 已选择组件\n\n本项目由脚手架生成，组件组合为 `" + strings.Join(selected, " + ") + "`。未选择的适配器和依赖已经移除。\n\n" + updated[end:]
	}
	if opts.database == "mysql" {
		updated = strings.ReplaceAll(updated, "PostgreSQL/MySQL", "MySQL")
		updated = strings.ReplaceAll(updated, "PostgreSQL", "MySQL")
		updated = strings.ReplaceAll(updated, "postgres://postgres:postgres@localhost:5432/app?sslmode=disable", "root:root@tcp(localhost:3306)/app?charset=utf8mb4&parseTime=true&loc=UTC")
	}
	updated = strings.ReplaceAll(updated, "未初始化的模板默认使用 PostgreSQL 和可选 Redis；生成项目会要求已选择的业务库和采集库提供 DSN", "本项目要求已选择的业务库和采集库提供 DSN")
	updated = strings.ReplaceAll(updated, "未初始化的模板默认使用 MySQL 和可选 Redis；生成项目会要求已选择的业务库和采集库提供 DSN", "本项目要求已选择的业务库和采集库提供 DSN")
	if opts.telemetry == "clickhouse" {
		updated = strings.ReplaceAll(updated, "ClickHouse/TDengine", "ClickHouse")
		updated = strings.ReplaceAll(updated, "ClickHouse 运行时使用官方 Native Client；TDengine 只使用 WebSocket Driver。", "ClickHouse 运行时使用官方 Native Client。")
		updated = strings.Replace(updated, "export REDIS_URL", "export CLICKHOUSE_DSN='clickhouse://app:app@localhost:9000/app?compress=lz4'\nexport REDIS_URL", 1)
	} else if opts.telemetry == "tdengine" {
		updated = strings.ReplaceAll(updated, "ClickHouse/TDengine", "TDengine")
		updated = strings.ReplaceAll(updated, "ClickHouse 运行时使用官方 Native Client；TDengine 只使用 WebSocket Driver。", "TDengine 运行时只使用官方 WebSocket Driver。")
		updated = strings.Replace(updated, "export REDIS_URL", "export TDENGINE_DSN='root:taosdata@ws(localhost:6041)/'\nexport REDIS_URL", 1)
	}
	updated = strings.ReplaceAll(updated, "DATABASE_URL", "DATABASE_DSN")
	lines := strings.Split(updated, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if (opts.database != "postgres" && strings.Contains(line, "migrations/postgres")) ||
			(opts.database != "mysql" && strings.Contains(line, "migrations/mysql")) ||
			(opts.telemetry != "clickhouse" && (strings.Contains(line, "platform/clickhouse") || strings.Contains(line, "migrations/clickhouse"))) ||
			(opts.telemetry != "tdengine" && (strings.Contains(line, "platform/tdengine") || strings.Contains(line, "migrations/tdengine"))) ||
			(!opts.redis && (strings.Contains(line, "platform/redis") || strings.Contains(line, "REDIS_URL"))) ||
			(!opts.mqtt && (strings.Contains(line, "platform/mqtt") || strings.Contains(line, "MQTT_") || strings.Contains(line, "选择 MQTT"))) ||
			(!opts.redisStream && (strings.Contains(line, "platform/redisstream") || strings.Contains(line, "Redis Stream 后"))) ||
			(opts.frontend != "embed" && (strings.Contains(line, "httpapi/frontend") || strings.Contains(line, "-frontend=embed"))) {
			continue
		}
		kept = append(kept, line)
	}
	updated = strings.Join(kept, "\n")
	return os.WriteFile(path, []byte(updated), 0o644)
}

func validate(opts *options) error {
	if opts.frontend == "" {
		opts.frontend = "none"
	}
	if err := module.CheckPath(opts.module); err != nil {
		return fmt.Errorf("invalid module path: %w", err)
	}
	if opts.name == "" {
		parts := strings.Split(opts.module, "/")
		opts.name = parts[len(parts)-1]
	}
	if !regexp.MustCompile("^[A-Za-z0-9][A-Za-z0-9._-]*$").MatchString(opts.name) {
		return fmt.Errorf("invalid service name %q", opts.name)
	}
	if opts.database != "postgres" && opts.database != "mysql" && opts.database != "none" {
		return fmt.Errorf("database must be postgres, mysql, or none")
	}
	if opts.telemetry != "clickhouse" && opts.telemetry != "tdengine" && opts.telemetry != "none" {
		return fmt.Errorf("telemetry must be clickhouse, tdengine, or none")
	}
	if opts.redisStream && !opts.redis {
		return fmt.Errorf("redis-stream requires redis")
	}
	if opts.frontend != "embed" && opts.frontend != "none" {
		return fmt.Errorf("frontend must be embed or none")
	}
	return nil
}

func deletionList(opts options) []string {
	paths := []string{"cmd/scaffold"}
	if opts.database == "none" {
		paths = append(paths, "cmd/migrate", "internal/httpapi/router_test.go", "internal/note/repository_test.go")
	}
	if opts.database != "postgres" {
		paths = append(paths, "internal/platform/database/postgres", "migrations/postgres")
	}
	if opts.database != "mysql" {
		paths = append(paths, "internal/platform/database/mysql", "migrations/mysql")
	}
	if opts.telemetry != "clickhouse" {
		paths = append(paths, "internal/platform/clickhouse", "migrations/clickhouse")
	}
	if opts.telemetry != "tdengine" {
		paths = append(paths, "internal/platform/tdengine", "migrations/tdengine")
	}
	if !opts.redis {
		paths = append(paths, "internal/platform/redis")
	}
	if !opts.mqtt {
		paths = append(paths, "internal/platform/mqtt", "deploy/mosquitto.conf")
	}
	if !opts.redisStream {
		paths = append(paths, "internal/platform/redisstream")
	}
	if opts.frontend != "embed" {
		paths = append(paths, "internal/httpapi/frontend")
	}
	return paths
}

func adaptRouter(root, frontend string) error {
	if frontend != "embed" {
		return nil
	}
	path := filepath.Join(root, "internal/httpapi/router.go")
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated := strings.Replace(string(contents), "\t\"go-web-template/internal/health\"\n", "\t\"go-web-template/internal/health\"\n\t\"go-web-template/internal/httpapi/frontend\"\n", 1)
	updated = strings.Replace(updated, "router.NoRoute(func(c *gin.Context) { response.Error(c, http.StatusNotFound, \"not found\") })", "router.NoRoute(frontend.Handler(dependencies.HTTPPrefix))", 1)
	if updated == string(contents) {
		return fmt.Errorf("adapt frontend router: expected template markers not found")
	}
	return os.WriteFile(path, []byte(updated), 0o644)
}

func replaceModule(root, modulePath, name string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		base := filepath.Base(path)
		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".mod" && ext != ".md" && ext != ".yaml" && ext != ".yml" && base != "Dockerfile" && base != "Makefile" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		updated := strings.ReplaceAll(string(contents), "go-web-template", modulePath)
		if base == "README.md" {
			updated = strings.Replace(updated, "# "+modulePath, "# "+name, 1)
		}
		if updated == string(contents) {
			return nil
		}
		return os.WriteFile(path, []byte(updated), 0o644)
	})
}

func filterExamples(root string, opts options) error {
	path := filepath.Join(root, "config.example.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(filterYAMLExample(string(contents), opts)), 0o644)
}

func filterYAMLExample(contents string, opts options) string {
	lines := strings.Split(contents, "\n")
	kept := lines[:0]
	section := ""
	skipSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(line) == len(strings.TrimLeft(line, " \t")) {
			section, _, _ = strings.Cut(trimmed, ":")
			skipSection = (section == "database" && opts.database == "none") ||
				(section == "telemetry" && opts.telemetry == "none") ||
				(section == "redis" && !opts.redis) ||
				(section == "mqtt" && !opts.mqtt)
		}
		if skipSection {
			continue
		}
		if section == "telemetry" && ((opts.telemetry != "clickhouse" && strings.HasPrefix(trimmed, "clickhouse_dsn:")) ||
			(opts.telemetry != "tdengine" && strings.HasPrefix(trimmed, "tdengine_dsn:"))) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func renderIntegrations(opts options) string {
	base := `package integration

import (
	"github.com/sirupsen/logrus"

	"go-web-template/internal/config"
	"go-web-template/internal/health"
)

type Integrations struct{}

func Open(config.Config, *logrus.Logger) (*Integrations, error) { return &Integrations{}, nil }
func (*Integrations) Checks() []health.Check                    { return nil }
func (*Integrations) Close() error                              { return nil }
`
	if !opts.mqtt {
		return base
	}
	return `package integration

import (
	"fmt"

	"github.com/sirupsen/logrus"

	"go-web-template/internal/config"
	"go-web-template/internal/health"
	mqttclient "go-web-template/internal/platform/mqtt"
)

type Integrations struct {
	MQTT *mqttclient.Client
	checks []health.Check
}

func Open(cfg config.Config, logger *logrus.Logger) (*Integrations, error) {
	if cfg.MQTT.URL == "" { return nil, fmt.Errorf("MQTT_URL is required") }
	client, err := mqttclient.Open(mqttclient.Options{
		URL: cfg.MQTT.URL, ClientID: cfg.MQTT.ClientID, Username: cfg.MQTT.Username, Password: cfg.MQTT.Password,
		ConnectTimeout: cfg.MQTT.ConnectTimeout, KeepAlive: cfg.MQTT.KeepAlive, Logger: logger,
	})
	if err != nil { return nil, err }
	return &Integrations{MQTT: client, checks: []health.Check{{Name: "mqtt", Ping: client.Ping}}}, nil
}

func (i *Integrations) Checks() []health.Check { return i.checks }
func (i *Integrations) Close() error { return i.MQTT.Close() }
`
}

func renderStorage(opts options) string {
	var imports, fields, open, migrate, checks, resources strings.Builder
	imports.WriteString("\"errors\"\n\t\"fmt\"\n\n\t\"github.com/sirupsen/logrus\"\n\n\t\"go-web-template/internal/config\"\n\t\"go-web-template/internal/health\"\n\t\"go-web-template/internal/platform/database\"\n")
	fields.WriteString("\tPrimary *database.Database\n")
	if opts.database == "postgres" {
		imports.WriteString("\tpostgresstore \"go-web-template/internal/platform/database/postgres\"\n\tpostgresmigrations \"go-web-template/migrations/postgres\"\n")
		open.WriteString(primaryOpen("postgresstore", "PostgreSQL", "postgresmigrations"))
		migrate.WriteString(primaryMigrate("postgresstore", "postgresmigrations"))
	} else if opts.database == "mysql" {
		imports.WriteString("\tmysqlstore \"go-web-template/internal/platform/database/mysql\"\n\tmysqlmigrations \"go-web-template/migrations/mysql\"\n")
		open.WriteString(primaryOpen("mysqlstore", "MySQL", "mysqlmigrations"))
		migrate.WriteString(primaryMigrate("mysqlstore", "mysqlmigrations"))
	} else {
		open.WriteString("\tprimary := database.Disabled()\n")
		migrate.WriteString("\treturn 0, false, fmt.Errorf(\"this project has no relational database\")\n")
	}
	resources.WriteString("primary")
	checks.WriteString("\tif primary.Enabled() { result.checks = append(result.checks, health.Check{Name: \"database\", Ping: primary.Ping}) }\n")
	if opts.telemetry == "clickhouse" {
		imports.WriteString("\tclickhousestore \"go-web-template/internal/platform/clickhouse\"\n")
		fields.WriteString("\tClickHouse *clickhousestore.ClickHouse\n")
		open.WriteString("\tif cfg.Telemetry.ClickHouseDSN == \"\" { closeAll([]closer{primary}); return nil, fmt.Errorf(\"CLICKHOUSE_DSN is required\") }\n\ttelemetry, err := clickhousestore.Open(cfg.Telemetry.ClickHouseDSN, clickhousestore.Options{MaxOpenConns: cfg.Telemetry.MaxOpenConns, MaxIdleConns: cfg.Telemetry.MaxIdleConns, ConnMaxLifetime: cfg.Telemetry.ConnMaxLifetime})\n\tif err != nil { closeAll([]closer{primary}); return nil, err }\n")
		resources.WriteString(", telemetry")
		checks.WriteString("\tresult.checks = append(result.checks, health.Check{Name: \"clickhouse\", Ping: telemetry.Ping})\n")
	} else if opts.telemetry == "tdengine" {
		imports.WriteString("\ttdenginestore \"go-web-template/internal/platform/tdengine\"\n")
		fields.WriteString("\tTDengine *tdenginestore.TDengine\n")
		open.WriteString("\tif cfg.Telemetry.TDengineDSN == \"\" { closeAll([]closer{primary}); return nil, fmt.Errorf(\"TDENGINE_DSN is required\") }\n\ttelemetry, err := tdenginestore.Open(cfg.Telemetry.TDengineDSN, tdenginestore.Options{MaxOpenConns: cfg.Telemetry.MaxOpenConns, MaxIdleConns: cfg.Telemetry.MaxIdleConns, ConnMaxLifetime: cfg.Telemetry.ConnMaxLifetime})\n\tif err != nil { closeAll([]closer{primary}); return nil, err }\n")
		resources.WriteString(", telemetry")
		checks.WriteString("\tresult.checks = append(result.checks, health.Check{Name: \"tdengine\", Ping: telemetry.Ping})\n")
	}
	if opts.redis {
		imports.WriteString("\tredisstore \"go-web-template/internal/platform/redis\"\n")
		fields.WriteString("\tRedis *redisstore.Redis\n")
		open.WriteString("\tredisClient, err := redisstore.Open(cfg.Redis.URL)\n\tif err != nil { closeAll([]closer{" + resources.String() + "}); return nil, err }\n")
		resources.WriteString(", redisClient")
		checks.WriteString("\tif redisClient.Enabled() { result.checks = append(result.checks, health.Check{Name: \"redis\", Ping: redisClient.Ping}) }\n")
	}
	assignments := "Primary: primary"
	if opts.telemetry == "clickhouse" {
		assignments += ", ClickHouse: telemetry"
	} else if opts.telemetry == "tdengine" {
		assignments += ", TDengine: telemetry"
	}
	if opts.redis {
		assignments += ", Redis: redisClient"
	}
	return "package storage\n\nimport (\n\t" + imports.String() + ")\n\ntype closer interface { Close() error }\n\ntype Storage struct {\n" + fields.String() + "\tchecks []health.Check\n\tclosers []closer\n}\n\nfunc Open(cfg config.Config, logger *logrus.Logger) (*Storage, error) {\n" + open.String() + "\tresult := &Storage{" + assignments + ", closers: []closer{" + resources.String() + "}}\n" + checks.String() + "\treturn result, nil\n}\n\nfunc (s *Storage) Checks() []health.Check { return s.checks }\n\nfunc (s *Storage) Close() error { return closeAll(s.closers) }\n\nfunc closeAll(resources []closer) error {\n\tvar errs []error\n\tfor i := len(resources)-1; i >= 0; i-- { if err := resources[i].Close(); err != nil { errs = append(errs, err) } }\n\treturn errors.Join(errs...)\n}\n\nfunc Migrate(cfg config.Config, logger *logrus.Logger, command string, steps int) (uint, bool, error) {\n" + migrate.String() + "}\n"
}

func primaryOpen(adapter, label, migrations string) string {
	return "\tif cfg.Database.DSN == \"\" { return nil, fmt.Errorf(\"DATABASE_DSN is required\") }\n\tprimary, err := " + adapter + ".Open(cfg.Database.DSN, database.Options{MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns, ConnMaxLifetime: cfg.Database.ConnMaxLifetime, SlowThreshold: cfg.Database.SlowThreshold, LogLevel: cfg.Database.LogLevel, Logger: logger})\n\tif err != nil { return nil, err }\n\tif cfg.Database.MigrateOnStart { if err := " + migrations + ".Up(primary.SQL()); err != nil { _ = primary.Close(); return nil, fmt.Errorf(\"migrate " + label + ": %w\", err) } }\n"
}

func primaryMigrate(adapter, migrations string) string {
	return "\tif cfg.Database.DSN == \"\" { return 0, false, fmt.Errorf(\"DATABASE_DSN is required\") }\n\tprimary, err := " + adapter + ".Open(cfg.Database.DSN, database.Options{MaxOpenConns: cfg.Database.MaxOpenConns, MaxIdleConns: cfg.Database.MaxIdleConns, ConnMaxLifetime: cfg.Database.ConnMaxLifetime, SlowThreshold: cfg.Database.SlowThreshold, LogLevel: cfg.Database.LogLevel, Logger: logger})\n\tif err != nil { return 0, false, err }\n\tdefer primary.Close()\n\tswitch command { case \"up\": err = " + migrations + ".Up(primary.SQL()); case \"down\": if steps <= 0 { return 0, false, fmt.Errorf(\"steps must be positive\") }; err = " + migrations + ".Down(primary.SQL(), steps); case \"version\": return " + migrations + ".Version(primary.SQL()); default: return 0, false, fmt.Errorf(\"unsupported migration command %q\", command) }\n\tif err != nil { return 0, false, err }; return " + migrations + ".Version(primary.SQL())\n"
}

func renderCompose(opts options) string {
	var services, volumes strings.Builder
	var appEnvironment, dependencies strings.Builder
	services.WriteString("services:\n")
	if opts.database == "postgres" {
		services.WriteString("  postgres:\n    image: postgres:17-alpine\n    environment:\n      POSTGRES_DB: app\n      POSTGRES_PASSWORD: postgres\n    ports: [\"5432:5432\"]\n    volumes: [\"postgres-data:/var/lib/postgresql/data\"]\n    healthcheck:\n      test: [\"CMD-SHELL\", \"pg_isready -U postgres -d app\"]\n      interval: 2s\n      timeout: 2s\n      retries: 15\n")
		volumes.WriteString("  postgres-data:\n")
		appEnvironment.WriteString("      DATABASE_DSN: postgres://postgres:postgres@postgres:5432/app?sslmode=disable\n")
		dependencies.WriteString("      postgres:\n        condition: service_healthy\n")
	} else if opts.database == "mysql" {
		services.WriteString("  mysql:\n    image: mysql:8.4\n    environment:\n      MYSQL_DATABASE: app\n      MYSQL_ROOT_PASSWORD: root\n    ports: [\"3306:3306\"]\n    volumes: [\"mysql-data:/var/lib/mysql\"]\n    healthcheck:\n      test: [\"CMD\", \"mysqladmin\", \"ping\", \"-h\", \"127.0.0.1\", \"-proot\"]\n      interval: 3s\n      timeout: 2s\n      retries: 30\n")
		volumes.WriteString("  mysql-data:\n")
		appEnvironment.WriteString("      DATABASE_DSN: root:root@tcp(mysql:3306)/app?charset=utf8mb4&parseTime=true&loc=UTC\n")
		dependencies.WriteString("      mysql:\n        condition: service_healthy\n")
	}
	if opts.telemetry == "clickhouse" {
		services.WriteString("  clickhouse:\n    image: clickhouse/clickhouse-server:25.8-alpine\n    environment:\n      CLICKHOUSE_DB: app\n      CLICKHOUSE_USER: app\n      CLICKHOUSE_PASSWORD: app\n    ports: [\"8123:8123\", \"9000:9000\"]\n    volumes: [\"clickhouse-data:/var/lib/clickhouse\"]\n    healthcheck:\n      test: [\"CMD-SHELL\", \"clickhouse-client --user app --password app --query 'SELECT 1'\"]\n      interval: 3s\n      timeout: 2s\n      retries: 30\n")
		volumes.WriteString("  clickhouse-data:\n")
		appEnvironment.WriteString("      CLICKHOUSE_DSN: clickhouse://app:app@clickhouse:9000/app?compress=lz4\n")
		dependencies.WriteString("      clickhouse:\n        condition: service_healthy\n")
	} else if opts.telemetry == "tdengine" {
		services.WriteString("  tdengine:\n    image: tdengine/tdengine:3.3.6.0\n    ports: [\"6041:6041\"]\n    volumes: [\"tdengine-data:/var/lib/taos\"]\n")
		volumes.WriteString("  tdengine-data:\n")
		appEnvironment.WriteString("      TDENGINE_DSN: root:taosdata@ws(tdengine:6041)/\n")
		dependencies.WriteString("      tdengine:\n        condition: service_started\n")
	}
	if opts.redis {
		services.WriteString("  redis:\n    image: redis:7-alpine\n    ports: [\"6379:6379\"]\n    healthcheck:\n      test: [\"CMD\", \"redis-cli\", \"ping\"]\n      interval: 2s\n      timeout: 2s\n      retries: 15\n")
		appEnvironment.WriteString("      REDIS_URL: redis://redis:6379/0\n")
		dependencies.WriteString("      redis:\n        condition: service_healthy\n")
	}
	if opts.mqtt {
		services.WriteString("  mosquitto:\n    image: eclipse-mosquitto:2\n    ports: [\"1883:1883\"]\n    volumes: [\"./deploy/mosquitto.conf:/mosquitto/config/mosquitto.conf:ro\"]\n")
		appEnvironment.WriteString("      MQTT_URL: tcp://mosquitto:1883\n      MQTT_CLIENT_ID: " + opts.name + "\n")
		dependencies.WriteString("      mosquitto:\n        condition: service_started\n")
	}
	services.WriteString("  app:\n    profiles: [\"app\"]\n    build:\n      context: .\n      args:\n        VERSION: ${VERSION:-dev}\n        COMMIT: ${COMMIT:-unknown}\n        BUILD_TIME: ${BUILD_TIME:-unknown}\n    environment:\n      HTTP_ADDR: \":8080\"\n")
	services.WriteString(appEnvironment.String())
	if dependencies.Len() > 0 {
		services.WriteString("    depends_on:\n" + dependencies.String())
	}
	services.WriteString("    ports: [\"8080:8080\"]\n    healthcheck:\n      test: [\"CMD\", \"wget\", \"-q\", \"-O\", \"-\", \"http://127.0.0.1:8080/ready\"]\n      interval: 10s\n      timeout: 2s\n      retries: 6\n")
	if volumes.Len() > 0 {
		services.WriteString("\nvolumes:\n" + volumes.String())
	}
	return services.String()
}
