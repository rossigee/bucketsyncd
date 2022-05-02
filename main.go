package main

import (
  "os"
  "fmt"
	"io/ioutil"
	"net/url"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"gopkg.in/yaml.v2"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanuber/go-glob"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/s3/s3manager"
)

var config Config

type Config struct {
	LogLevel string `yaml:"log_level"`
	LogJSON  bool   `yaml:"log_json"`
	Outbound []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Sensitive   bool   `yaml:"sensitive"`
		Source      string `yaml:"source"`
		Destination string `yaml:"destination"`
		ProcessWith string `yaml:"process_with,omitempty"`
	} `yaml:"outbound"`
	Inbound []struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
		Source      string `yaml:"source"`
		Destination string `yaml:"destination"`
	} `yaml:"inbound"`
	Notifications []struct {
		Name   string `yaml:"name"`
		Method string `yaml:"method"`
		URL    string `yaml:"url"`
	} `yaml:"notifications"`
}

func main() {
	// Read YAML config file
	filename, _ := filepath.Abs("./config.yaml")
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(yamlFile, &config)
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

  // Configure AWS connections
  var awsConfig *aws.Config = &aws.Config{
    Region: aws.String("ap-southeast-1"),
  }
  sess := session.Must(session.NewSession(awsConfig))
  uploader := s3manager.NewUploader(sess)

	// Channel should respond by checking whether active watchers still exist
	watchers := []fsnotify.Watcher{}

	// Stops the program from exiting prematurely
	done := make(chan bool)

	// Set up watcher for each source
	for i := 0; i < len(config.Outbound); i++ {
		o := config.Outbound[i]
		lf := log.Fields{
			"workflow": o.Name,
		}
		log.WithFields(lf).Info("configuring watcher for '", o.Description, "'")

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.WithFields(lf).Fatal(err)
		}
		defer watcher.Close()
		watchers = append(watchers, *watcher)

		// Extract folder to watch, and file glob to filter on
		localFolder := filepath.Dir(o.Source)
		fileGlob := filepath.Base(o.Source)
		log.WithFields(lf).WithFields(log.Fields{
			"folder":   localFolder,
			"fileglob": fileGlob,
		}).Debug("")

		// Define function to handle events
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}

					log.WithFields(lf).WithFields(log.Fields{
						"name": event.Name,
						"op":   event.Op,
					}).Debug("Event")

					// Ignore non-Write events
					if event.Op&fsnotify.Write != fsnotify.Write {
						log.WithFields(lf).WithFields(log.Fields{
							"name": event.Name,
							"op":   event.Op,
						}).Debug("Ignoring unimportant event type")
						continue
					}

					// Does filename match the fileglob?
					filename := filepath.Base(event.Name)
					if !glob.Glob(fileGlob, filename) {
						log.WithFields(lf).WithFields(log.Fields{
							"name": event.Name,
							"op":   event.Op,
						}).Debug("Ignoring write event due to glob mismatch")
						continue
					}

					// Open the file and prepare to reads it
          f, err := os.Open(event.Name)
					if o.ProcessWith != "" {
						if err != nil {
              log.WithFields(lf).WithFields(log.Fields{
                "name": event.Name,
                "op":   event.Op,
              }).Error(fmt.Printf("failed to open file %q, %v", filename, err))
							return
						}
          }
          defer f.Close()
          
          // [TODO] If we need to stream to a processor, do so here
          if o.ProcessWith != "" {
            log.WithFields(lf).WithFields(log.Fields{
              "name": event.Name,
              "op":   event.Op,
            }).Error("NOT IMPLEMENTED")
            return
          }

					// Stream output from file/processor to S3
					u, err := url.Parse(o.Destination)
					awsBucket := u.Hostname()
					awsFileKey := u.Path + "/" + filename
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
					}).Info("sending to S3")
					result, err := uploader.Upload(&s3manager.UploadInput{
						Bucket: aws.String(awsBucket),
						Key:    aws.String(awsFileKey),
						Body:   f,
					})
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
              "result":    result,
						}).Error("failed to upload file to S3")
						return
					}
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
					}).Info("failed uploaded to S3")

				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				}
			}
		}()

		// Start watching folder
		err = watcher.Add(localFolder)
		if err != nil {
			log.Fatal(err)
		}

	}

	<-done
}
