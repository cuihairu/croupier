package config

import (
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
)

type DatabaseConfig struct {
	Type  string
	Mysql mysql.Config
}

type Config struct {
	DB DatabaseConfig
}

func LoadConfig(configFile string) (*Config, error) {
	if len(configFile) > 0 {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath("./config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.croupier")
		viper.SetConfigName("config")
	}
	var cfg Config
	err := viper.Unmarshal(&cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
