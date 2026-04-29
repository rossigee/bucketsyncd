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
	version    = "v0.4.0"
	buildTime  = "unknown"
	gitCommit  = "unknown"
)

const (
	debugLevel = "debug"
	infoLevel  = "info"
	warnLevel  = "warn"
)

var (
	configFilePath = flag.String("c", "", "Configuration file location")
	help           = flag.Bool("h", false, "Usage information")
	showVersion    = flag.Bool("version", false, "Show version information")
)

func main() {
	// Parse command line arguments and handle help/usage
	if !parseCommandLine() {
		return
	}

	// Read YAML config file
	err := readConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	log.Info(fmt.Sprintf("Loaded %d remotes", len(config.Remotes)))

	// Configure logging
	configureLogging()

	log.Info("starting bucketsyncd")
	log.Info(fmt.Sprintf("build info: version=%s build_time=%s git_commit=%s", version, buildTime, gitCommit))

	// Start processing
	runService()
}

func parseCommandLine() bool {
	flag.Parse()

	if *showVersion {
		fmt.Println("bucketsyncd", version)
		return false
	}

	if *configFilePath == "" {
		fmt.Println("Error: -c option is required")
	}
	if *help || *configFilePath == "" {
		fmt.Println("Usage:", os.Args[0], " [-c <config_file_path>] [-h] [-version]")
		return false
	}
	return true
}

func configureLogging() {
	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	switch config.LogLevel {
	case debugLevel:
		log.SetLevel(log.DebugLevel)
	case infoLevel:
		log.SetLevel(log.InfoLevel)
	case warnLevel:
		log.SetLevel(log.WarnLevel)
	}
	if config.LogLevel == debugLevel {
		log.SetLevel(log.DebugLevel)
	}
	if config.LogJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func runService() {
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
	const signalBufferSize = 2
	c := make(chan os.Signal, signalBufferSize)
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
