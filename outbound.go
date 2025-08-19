package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanuber/go-glob"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var watchers []fsnotify.Watcher

// nolint:gocognit,funlen // This function handles the main file watching and upload logic
func outbound(o Outbound) {
	lf := log.Fields{
		"workflow": o.Name,
	}
	log.WithFields(lf).Info("configuring watcher for '", o.Description, "'")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.WithFields(lf).Error(err)
		return
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			log.WithFields(lf).Error("failed to close watcher: ", err)
		}
	}()
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
				defer func() {
					if err := f.Close(); err != nil {
						log.WithFields(lf).Error("failed to close file: ", err)
					}
				}()

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
				// p.Flush()

				// Create a buffered reader

				// Determine remote bucket details
				u, err := url.Parse(o.Destination)
				if err != nil {
					log.WithFields(lf).Error("failed to parse destination URL: ", err)
					return
				}
				endpoint := u.Hostname()
				tokens := strings.Split(u.Path, "/")
				const minTokens = 2
				if len(tokens) < minTokens {
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
					log.WithFields(lf).Error("No credentials found")
					return
				}
				mc, err := minio.New(endpoint, &minio.Options{
					Creds:  &creds,
					Secure: true,
				})
				if err != nil {
					log.WithFields(lf).Fatal(err)
					return
				}

				// Push object to bucket
				fs, err := f.Stat()
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
		log.WithFields(lf).Fatal(err)
	}
}
