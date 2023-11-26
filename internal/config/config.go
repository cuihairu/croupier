package config

import (
	"fmt"
	"github.com/spf13/viper"
	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"os"
)

type DatabaseConfig struct {
	Type       string            `json:"type" yaml:"type" default:"mysql"`
	Mysql      mysql.Config      `json:"mysql" yaml:"mysql"`
	Sqlite     sqlite.Dialector  `json:"sqlite" yaml:"sqlite"`
	Clickhouse clickhouse.Config `json:"clickhouse" yaml:"clickhouse"`
	Postgres   postgres.Config   `json:"postgres" yaml:"postgres"`
	Sqlserver  sqlserver.Config  `json:"sqlserver" yaml:"sqlserver"`
}

type Config struct {
	Debug bool
	DB    DatabaseConfig `json:"DB" yaml:"DB"`
}

func LoadConfig(configFile string, debug bool) (*Config, error) {
	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath("config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("etc/")
		viper.AddConfigPath("$HOME/.croupier")
		viper.SetConfigName("config")
	}
	if debug {
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		fmt.Printf("work dir: %s", dir)
	}

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	if debug {
		fmt.Printf("config file used: %s", viper.ConfigFileUsed())
	}
	var cfg = &Config{
		DB: DatabaseConfig{
			Type: "mysql",
		},
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.Debug = debug || cfg.Debug
	return cfg, nil
}
