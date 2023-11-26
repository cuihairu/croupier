package internal

import (
	"fmt"
	"github.com/chuihairu/croupier/internal/config"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

var (
	app  *ServerApplication
	once sync.Once
)

type ServerApplication struct {
	config *config.Config
	db     *gorm.DB
}

func ServerApplicationInstance() *ServerApplication {
	once.Do(func() {
		app = &ServerApplication{}
	})
	return app
}

func (s *ServerApplication) LoadConfig(configFile string, debug bool) error {
	loadConfig, err := config.LoadConfig(configFile, debug)
	if err != nil {
		return err
	}
	s.config = loadConfig
	return nil
}

func (s *ServerApplication) SaveConfig(configFile string) error {
	return config.SaveConfig(configFile)
}

func (s *ServerApplication) Run() {
	for {
		log.Printf("runing")
		time.Sleep(3 * time.Second)
		if s.db != nil {
			fmt.Print("db init")
		}
	}
}
