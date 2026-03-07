package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const defaultConfigPath = "app.yml"

type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Database DatabaseConfig `koanf:"database"`
}

type ServerConfig struct {
	Host                string `koanf:"host"`
	Port                int    `koanf:"port"`
	ReadTimeoutSeconds  int    `koanf:"readTimeoutSeconds"`
	WriteTimeoutSeconds int    `koanf:"writeTimeoutSeconds"`
	IdleTimeoutSeconds  int    `koanf:"idleTimeoutSeconds"`
}

type DatabaseConfig struct {
	Host                   string `koanf:"host"`
	Port                   int    `koanf:"port"`
	Name                   string `koanf:"name"`
	User                   string `koanf:"user"`
	Password               string `koanf:"password"`
	SSLMode                string `koanf:"sslmode"`
	MaxConns               int32  `koanf:"maxConns"`
	MinConns               int32  `koanf:"minConns"`
	MaxConnIdleTimeSeconds int    `koanf:"maxConnIdleTimeSeconds"`
	MaxConnLifetimeSeconds int    `koanf:"maxConnLifetimeSeconds"`
}

func Load() (Config, error) {
	configPath := os.Getenv("APP_CONFIG")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	k := koanf.New(".")
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		return Config{}, fmt.Errorf("load %s: %w", configPath, err)
	}

	if err := k.Load(env.Provider("APP_", ".", func(value string) string {
		return strings.ToLower(strings.ReplaceAll(value, "_", "."))
	}), nil); err != nil {
		return Config{}, fmt.Errorf("load env overrides: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) validate() error {
	if c.Server.Host == "" {
		return fmt.Errorf("server.host is required")
	}
	if c.Server.Port <= 0 {
		return fmt.Errorf("server.port must be > 0")
	}
	if c.Database.Host == "" || c.Database.Name == "" || c.Database.User == "" {
		return fmt.Errorf("database host, name and user are required")
	}
	if c.Database.Port <= 0 {
		return fmt.Errorf("database.port must be > 0")
	}
	if c.Database.MaxConns <= 0 {
		return fmt.Errorf("database.maxConns must be > 0")
	}
	if c.Database.MinConns < 0 {
		return fmt.Errorf("database.minConns must be >= 0")
	}
	if c.Database.MinConns > c.Database.MaxConns {
		return fmt.Errorf("database.minConns must be <= database.maxConns")
	}

	return nil
}

func (c ServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c ServerConfig) ReadTimeout() time.Duration {
	return time.Duration(c.ReadTimeoutSeconds) * time.Second
}

func (c ServerConfig) WriteTimeout() time.Duration {
	return time.Duration(c.WriteTimeoutSeconds) * time.Second
}

func (c ServerConfig) IdleTimeout() time.Duration {
	return time.Duration(c.IdleTimeoutSeconds) * time.Second
}

func (c DatabaseConfig) MaxConnIdleTime() time.Duration {
	return time.Duration(c.MaxConnIdleTimeSeconds) * time.Second
}

func (c DatabaseConfig) MaxConnLifetime() time.Duration {
	return time.Duration(c.MaxConnLifetimeSeconds) * time.Second
}

func (c DatabaseConfig) ConnString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host,
		c.Port,
		c.Name,
		c.User,
		c.Password,
		c.SSLMode,
	)
}

func (c DatabaseConfig) ApplyPoolConfig(poolConfig *pgxpool.Config) {
	poolConfig.MaxConns = c.MaxConns
	poolConfig.MinConns = c.MinConns
	poolConfig.MaxConnIdleTime = c.MaxConnIdleTime()
	poolConfig.MaxConnLifetime = c.MaxConnLifetime()
}
