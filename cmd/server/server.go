package main

import (
	"log"
	"time"
)

type ServerApplication struct {
}

func newServerApplication() *ServerApplication {
	return &ServerApplication{}
}

func (s *ServerApplication) Run() {
	for {
		log.Printf("runing")
		time.Sleep(3 * time.Second)
	}
}
