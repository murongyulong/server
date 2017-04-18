package main

import (
	"flag"
	"fmt"
	"github.com/murongyulong/server/cron"
	"github.com/murongyulong/server/g"
	"github.com/murongyulong/server/hbs"
	"github.com/murongyulong/server/http"
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
