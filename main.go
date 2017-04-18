package main

import (
	"flag"
	"fmt"
	"github.com/smartcaas/server/cron"
	"github.com/smartcaas/server/g"
	"github.com/smartcaas/server/hbs"
	"github.com/smartcaas/server/http"
	"os"
)

func main() {
	cfg := flag.String("c", "cfg.json", "configuration file")
	version := flag.Bool("v", false, "show version")
	flag.Parse()

	if *version {
		fmt.Println(g.VERSION)
		os.Exit(0)
	}

	g.ParseConfig(*cfg)

	g.InitRedisConnPool()
	g.InitDbConnPool()

	go cron.CompareState()
	go cron.CheckStale()
	go cron.SyncRoutes()
	go cron.SyncDomain()

	go http.Start()
	hbs.Start()
}
