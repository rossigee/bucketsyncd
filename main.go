package main

import (
	"fmt"

	"os"

	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"
)

var watchers []fsnotify.Watcher

func main() {
	// Parse command line arguments
	var configFilePath = flag.String("c", "", "Configuration file location")
	var help = flag.Bool("h", false, "Usage information")
	flag.Parse()
	if *configFilePath == "" {
		fmt.Println("Error: -c option is required")
	}
	if *help || *configFilePath == "" {
		fmt.Println("Usage:", os.Args[0], " [-c <config_file_path>] [-h]")
		return
	}

	// Read YAML config file
	err := readConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	// Configure logging
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})
	switch config.LogLevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		log.SetLevel(log.WarnLevel)
	}
	if config.LogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	if config.LogJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}

	// Stops the program from exiting prematurely
	done := make(chan bool)

	// Set up watcher for each outbound source
	for i := 0; i < len(config.Outbound); i++ {
		o := config.Outbound[i]
		outbound(o)
	}

	// Set up watcher for each inbound source
	for i := 0; i < len(config.Inbound); i++ {
		in := config.Inbound[i]
		inbound(in)
	}

	<-done
}
