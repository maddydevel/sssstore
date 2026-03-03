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
	case "doctor":
		doctorCmd(os.Args[2:])
	case "user":
		userCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println("sssstore commands: init, server, doctor, user")
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
	if err := server.Run(cfg); err != nil {
		log.Fatal(err)
	}
}

func doctorCmd(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	cfgPath := fs.String("config", "./sssstore.json", "Path to config file")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	if st, err := os.Stat(cfg.DataDir); err != nil || !st.IsDir() {
		log.Fatalf("data directory check failed: %v", err)
	}
	if st, err := os.Stat(cfg.DataDir + "/buckets"); err != nil || !st.IsDir() {
		log.Fatalf("buckets directory check failed: %v", err)
	}
	fmt.Println("doctor: ok")
	fmt.Printf("data_dir=%s\n", cfg.DataDir)
}

func userCmd(args []string) {
	if len(args) == 0 || args[0] != "create" {
		log.Fatal("usage: sssstore user create --config <path> --name <name> --access-key <key> --secret-key <secret>")
	}
	fs := flag.NewFlagSet("user create", flag.ExitOnError)
	cfgPath := fs.String("config", "./sssstore.json", "Path to config file")
	name := fs.String("name", "", "User name")
	accessKey := fs.String("access-key", "", "Access key")
	secretKey := fs.String("secret-key", "", "Secret key")
	_ = fs.Parse(args[1:])
	if *name == "" || *accessKey == "" || *secretKey == "" {
		log.Fatal("name, access-key and secret-key are required")
	}
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	users, err := config.LoadUsers(cfg.DataDir)
	if err != nil {
		log.Fatal(err)
	}
	users = append(users, config.User{Name: *name, AccessKey: *accessKey, SecretKey: *secretKey})
	if err := config.SaveUsers(cfg.DataDir, users); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("created user %s\n", *name)
}
