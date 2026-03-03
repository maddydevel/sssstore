package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/sssstore/sssstore/internal/config"
	"github.com/sssstore/sssstore/internal/server"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		initCmd(os.Args[2:])
	case "server":
		serverCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("sssstore commands: init, server")
}

func initCmd(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	cfgPath := fs.String("config", "./sssstore.json", "Path to config file")
	dataDir := fs.String("data", "./data", "Path to data directory")
	_ = fs.Parse(args)

	cfg, err := config.Init(*cfgPath, *dataDir)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("initialized config at %s\n", *cfgPath)
	fmt.Printf("bind_addr=%s data_dir=%s\n", cfg.BindAddr, cfg.DataDir)
}

func serverCmd(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	cfgPath := fs.String("config", "./sssstore.json", "Path to config file")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := server.Run(cfg.BindAddr, cfg.DataDir); err != nil {
		log.Fatal(err)
	}
}
