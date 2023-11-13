package internal

import (
	"fmt"
	"github.com/chuihairu/croupier/internal/config"
	"gorm.io/gorm"
	"log"
	"sync"
	"time"
)

var app *ServerApplication

type ServerApplication struct {
	config *config.Config
	db     *gorm.DB
}

func ServerApplicationInstance() *ServerApplication {
	sync.OnceFunc(func() {
		app = &ServerApplication{}
	})
	return app
}

func (s *ServerApplication) LoadConfig(configFile string) error {
	loadConfig, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	s.config = loadConfig
	return nil
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
