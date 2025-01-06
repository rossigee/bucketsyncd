package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	// "net/url"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanuber/go-glob"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	// Read YAML config file
	err := readConfig("./config.yaml")
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

	// Channel should respond by checking whether active watchers still exist
	watchers := []fsnotify.Watcher{}

	// Stops the program from exiting prematurely
	done := make(chan bool)

	// Set up watcher for each outbound source
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

					// Open the file and prepare to read it
					f, err := os.Open(event.Name)
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name": event.Name,
							"op":   event.Op,
						}).Error(fmt.Printf("failed to open file %q, %v", filename, err))
						return
					}
					defer f.Close()

					// [IGNORE THIS FOR NOW] If we need to stream to a processor, do so here
					// var p io.Writer = bufio.NewWriterSize(f, 1024)
					// if o.ProcessWith != "" {
					// 	cmd := exec.Command(o.ProcessWith)
					// 	cmd.Stdin = f
					// 	//stdout, err := cmd.Output()
					// 	cmd.Stdout = p
					// 	err := cmd.Start()
					// 	if err != nil {
					// 		// Handle error
					// 		log.WithFields(lf).WithFields(log.Fields{
					// 			"name":   event.Name,
					// 			"op":     event.Op,
					// 			"parser": o.ProcessWith,
					// 		}).Error("Parser error: ", err)
					// 		return
					// 	}
					// 	// Report success
					// 	log.WithFields(lf).WithFields(log.Fields{
					// 		"name":   event.Name,
					// 		"op":     event.Op,
					// 		"parser": o.ProcessWith,
					// 	}).Error("Parsed successfully")

					// } else {
					// 	// Pass through unprocessed
					// 	p = f
					// }
					//p.Flush()

					// Create a buffered reader

					// Stream output from file/processor to S3
					u, err := url.Parse(o.Destination)
					endpoint := u.Hostname()
					tokens := strings.Split(u.Path, "/")
					if len(tokens) < 2 {
						log.WithFields(lf).Error("Invalid S3 path: ", u.Path)
						return
					}
					awsBucket := tokens[1]
					awsFileKey := strings.Join(tokens[2:], "/") + "/" + filename
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"endpoint":   endpoint,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
					}).Debug("uploading to bucket")

					// Determine remote to use to create a new MinIO client
					creds := credentials.Credentials{}
					credsFound := false
					for _, remote := range config.Remotes {
						if remote.Endpoint == endpoint {
							creds = *credentials.NewStaticV4(remote.AccessKey, remote.SecretKey, "")
							credsFound = true
						}
					}
					if !credsFound {
						log.Fatal("No credentials found")
					}
					mc, err := minio.New(endpoint, &minio.Options{
						Creds:  &creds,
						Secure: true,
					})
					if err != nil {
						log.Fatal(err)
					}

					// Push object to bucket
					fs, _ := f.Stat()
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
						}).Error("unable to query file size: ", err)
						return
					}
					ctx := context.TODO()
					_, err = mc.PutObject(ctx, awsBucket, awsFileKey, f, fs.Size(), minio.PutObjectOptions{})
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
						}).Error("failed to upload file to S3: ", err)
						return
					}
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
						"size":       fs.Size(),
					}).Info("uploaded to S3")

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
