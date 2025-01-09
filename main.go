package main

import (
	"fmt"
	"os/signal"
	"syscall"

	"os"

	"flag"

	log "github.com/sirupsen/logrus"
)

var (
	configFilePath = flag.String("c", "", "Configuration file location")
	help           = flag.Bool("h", false, "Usage information")
)

func init() {
	flag.Parse()
}

func main() {
	// Parse command line arguments
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

	// Handle termination gracefully
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info("SIGTERM termination signal received")

		// Close AMQP connections
		inboundClose()

		done <- true
	}()

	<-done
}
