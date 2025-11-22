package main

import (
	"flag"
	"fmt"

	"github.com/cuihairu/croupier/services/edge/internal/config"
	"github.com/cuihairu/croupier/services/edge/internal/handler"
	"github.com/cuihairu/croupier/services/edge/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/edge.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting edge server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}