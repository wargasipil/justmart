package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Server struct {
	// Host is the interface to bind. "0.0.0.0" = all interfaces (LAN/Docker),
	// "127.0.0.1" = this machine only. Empty defaults to "0.0.0.0" (see Load).
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type Database struct {
	// Driver selects the database engine: "postgres" (default) or "sqlite".
	// Postgres is the multi-user/server deployment; sqlite is the turnkey,
	// zero-dependency single-binary flavor (Path below points at the file).
	Driver string `yaml:"driver"`
	// Path is the SQLite database file (e.g. "./justmart.db"). Only used when
	// Driver is "sqlite". Empty defaults to "./justmart.db".
	Path     string `yaml:"path"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
	SSLMode  string `yaml:"sslmode"`
	// AutoMigrate runs goose migrations on server boot. Pointer so unset is
	// distinguishable; the default is OFF (migrations are run explicitly via
	// `cmd/server migrate`). Set `auto_migrate: true` to migrate on boot — the
	// turnkey Docker / Windows configs do this. Read via ShouldAutoMigrate.
	AutoMigrate *bool `yaml:"auto_migrate"`
}

// DriverName normalizes the configured engine; empty => "postgres" (back-compat
// with existing configs that predate the driver selector).
func (d Database) DriverName() string {
	if d.Driver == "" {
		return "postgres"
	}
	return d.Driver
}

// IsSQLite reports whether the configured engine is SQLite.
func (d Database) IsSQLite() bool { return d.DriverName() == "sqlite" }

// SQLitePath returns the configured file path, defaulting to ./justmart.db.
func (d Database) SQLitePath() string {
	if d.Path == "" {
		return "./justmart.db"
	}
	return d.Path
}

// ShouldAutoMigrate reports whether the server should run migrations on boot.
// Defaults to FALSE when unset — the server does not migrate on start unless
// `auto_migrate: true` is set (turnkey deploys). Otherwise run `cmd/server migrate`.
func (d Database) ShouldAutoMigrate() bool {
	return d.AutoMigrate != nil && *d.AutoMigrate
}

type Auth struct {
	JWTSecret       string        `yaml:"jwt_secret"`
	AccessTokenTTL  time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL time.Duration `yaml:"refresh_token_ttl"`
}

type Bootstrap struct {
	OwnerEmail    string `yaml:"owner_email"`
	OwnerPassword string `yaml:"owner_password"`
}

type Printer struct {
	Enabled bool          `yaml:"enabled"`
	Address string        `yaml:"address"`        // host:port (raw TCP, typically port 9100)
	Width   int           `yaml:"width"`          // chars per line (32 for 58mm, 48 for 80mm)
	Timeout time.Duration `yaml:"timeout"`        // dial+write timeout
	Header  []string      `yaml:"header"`         // shop name/address lines printed on top
	Footer  []string      `yaml:"footer"`         // closing lines (e.g. "Thank you!")
	OpenDrawer bool       `yaml:"open_drawer"`    // send drawer-kick command after print
}

// Backup controls where BackupService writes per-timestamp backup directories.
// Empty Directory defaults to ./backups (CWD-relative, matches the legacy
// `make backup` behavior). Docker uses /var/lib/justmart/backups; Windows uses
// C:\ProgramData\Justmart\backups.
//
// PgToolsDir is where the in-app Create-backup feature caches the pg_dump
// binary it auto-downloads when none is found on PATH / bundled next to the
// justmart binary. Empty defaults to <UserCacheDir>/justmart/pgtools
// (Windows → %LOCALAPPDATA%\justmart\pgtools; Linux → ~/.cache/justmart/pgtools).
// Docker never triggers the auto-download (pg_dump is on PATH there), so this
// dir stays empty in containers.
type Backup struct {
	Directory  string `yaml:"directory"`
	PgToolsDir string `yaml:"pg_tools_dir"`
}

// Connector controls how SaleService.PrintReceipt dispatches the rendered
// receipt. Mode "tcp" (default) keeps the legacy raw-TCP-to-IP:9100 path
// (printer.Address). Mode "connector" pushes the rendered bytes to a connected
// print connector (a separate program by the printer; see cmd/connector). Token
// is the shared secret a connector must present on its Connect stream.
type Connector struct {
	Mode  string `yaml:"mode"`  // "tcp" (default) | "connector"
	Token string `yaml:"token"` // shared secret connectors must present
}

type Config struct {
	Server    Server    `yaml:"server"`
	Database  Database  `yaml:"database"`
	Auth      Auth      `yaml:"auth"`
	Bootstrap Bootstrap `yaml:"bootstrap"`
	Printer   Printer   `yaml:"printer"`
	Connector Connector `yaml:"connector"`
	Backup    Backup    `yaml:"backup"`
	// License is an offline license token (JWT minted by cmd/license, signed with
	// security.SecretRoot). When present + valid, its business type drives the
	// app's business mode on boot. Empty = unlicensed (mode stays UNSPECIFIED).
	License string `yaml:"license"`
}

func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// SQLiteDSN builds the modernc/glebarez DSN. PRAGMAs are set per-connection via
// the `_pragma` query params: foreign_keys ON (FK CASCADE relies on it), WAL
// (concurrent readers + one writer), and a busy_timeout so concurrent write
// transactions retry instead of erroring with "database is locked".
func (d Database) SQLiteDSN() string {
	return fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		d.SQLitePath(),
	)
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv("JUSTMART_CONFIG")
		if path == "" {
			path = "config.yaml"
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	var c Config
	if err := yaml.NewDecoder(f).Decode(&c); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	applyEnvOverrides(&c)
	applyDefaults(&c)
	return &c, nil
}

// applyEnvOverrides lets the most security-sensitive fields be supplied via
// environment variables instead of the YAML file. This keeps secrets out of a
// baked Docker image (12-factor) while the YAML still drives everything else.
// An empty env var is treated as "not set" and leaves the YAML value intact.
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("JUSTMART_JWT_SECRET"); v != "" {
		c.Auth.JWTSecret = v
	}
	if v := os.Getenv("JUSTMART_DB_DRIVER"); v != "" {
		c.Database.Driver = v
	}
	if v := os.Getenv("JUSTMART_DB_PATH"); v != "" {
		c.Database.Path = v
	}
	if v := os.Getenv("JUSTMART_DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("JUSTMART_DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("JUSTMART_OWNER_EMAIL"); v != "" {
		c.Bootstrap.OwnerEmail = v
	}
	if v := os.Getenv("JUSTMART_OWNER_PASSWORD"); v != "" {
		c.Bootstrap.OwnerPassword = v
	}
	if v := os.Getenv("JUSTMART_BACKUP_DIR"); v != "" {
		c.Backup.Directory = v
	}
	if v := os.Getenv("JUSTMART_PG_TOOLS_DIR"); v != "" {
		c.Backup.PgToolsDir = v
	}
	if v := os.Getenv("JUSTMART_LICENSE"); v != "" {
		c.License = v
	}
	if v := os.Getenv("JUSTMART_CONNECTOR_TOKEN"); v != "" {
		c.Connector.Token = v
	}
}

// applyDefaults fills in safe fallbacks for fields that the packaged flavors
// rely on. Bind to all interfaces by default so a container is reachable from
// the host; single-PC installs override this to 127.0.0.1.
func applyDefaults(c *Config) {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Connector.Mode == "" {
		c.Connector.Mode = "tcp" // legacy raw-TCP path stays the default
	}
	if c.Backup.Directory == "" {
		c.Backup.Directory = "./backups"
	}
	if c.Backup.PgToolsDir == "" {
		// os.UserCacheDir picks the right per-user cache root on each OS
		// (Windows: %LOCALAPPDATA%; Linux: $XDG_CACHE_HOME or ~/.cache;
		// macOS: ~/Library/Caches). Falls back to the backup directory if
		// the OS doesn't expose a cache root.
		if base, err := os.UserCacheDir(); err == nil {
			c.Backup.PgToolsDir = base + string(os.PathSeparator) + "justmart" + string(os.PathSeparator) + "pgtools"
		} else {
			c.Backup.PgToolsDir = c.Backup.Directory + string(os.PathSeparator) + "_pgtools"
		}
	}
}

func MustLoad() *Config {
	c, err := Load("")
	if err != nil {
		panic(err)
	}
	return c
}
