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

	configMutex.RLock()
	log.Info(fmt.Sprintf("Loaded %d remotes", len(config.Remotes)))
	configMutex.RUnlock()

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
	configMutex.RLock()
	logLevel := config.LogLevel
	logJSON := config.LogJSON
	configMutex.RUnlock()

	log.SetFormatter(&log.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	switch logLevel {
	case debugLevel:
		log.SetLevel(log.DebugLevel)
	case infoLevel:
		log.SetLevel(log.InfoLevel)
	case warnLevel:
		log.SetLevel(log.WarnLevel)
	}
	if logLevel == debugLevel {
		log.SetLevel(log.DebugLevel)
	}
	if logJSON {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func runService() {
	// Stops the program from exiting prematurely
	done := make(chan bool)

	configMutex.RLock()
	outboundConfigs := make([]Outbound, len(config.Outbound))
	copy(outboundConfigs, config.Outbound)
	inboundConfigs := make([]Inbound, len(config.Inbound))
	copy(inboundConfigs, config.Inbound)
	configMutex.RUnlock()

	// Set up watcher for each outbound source
	for i := 0; i < len(outboundConfigs); i++ {
		o := outboundConfigs[i]
		outbound(o)
	}

	// Set up watcher for each inbound source
	for i := 0; i < len(inboundConfigs); i++ {
		in := inboundConfigs[i]
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
