package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanuber/go-glob"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var watchers []*fsnotify.Watcher

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

	watchers = append(watchers, watcher)

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

				log.Info(fmt.Sprintf("Event received: name=%s op=%d", event.Name, event.Op))

				// Ignore non-Write/Create events
				if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
					log.Info(fmt.Sprintf("Ignoring event: name=%s op=%d", event.Name, event.Op))
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

				// Skip ignored files
				ignored := false
				for _, pattern := range o.IgnorePatterns {
					if glob.Glob(pattern, filename) {
						ignored = true
						break
					}
				}
				if ignored {
					log.WithFields(lf).WithFields(log.Fields{
						"name": event.Name,
						"op":   event.Op,
					}).Debug("Ignoring file due to ignore pattern")
					continue
				}

				// Open the file and prepare to read it
				// #nosec G304 - intentional: path comes from fsnotify watching a configured directory
				f, err := os.Open(event.Name)
				if err != nil {
					log.WithFields(lf).WithFields(log.Fields{
						"name": event.Name,
						"op":   event.Op,
					}).Error("failed to open file: ", err)
					continue
				}

				// Determine destination type and handle accordingly
				u, err := url.Parse(o.Destination)
				if err != nil {
					if closeErr := f.Close(); closeErr != nil {
						log.WithFields(lf).Error("failed to close file: ", closeErr)
					}
					log.WithFields(lf).Error("failed to parse destination URL: ", err)
					continue
				}

				// Check if this is a WebDAV destination
				if isWebDAVScheme(u.Scheme) {
					// Handle WebDAV upload
					webdavClient, err := NewWebDAVClient(o.Destination)
					if err != nil {
						if closeErr := f.Close(); closeErr != nil {
							log.WithFields(lf).Error("failed to close file: ", closeErr)
						}
						log.WithFields(lf).Error("failed to create WebDAV client: ", err)
						continue
					}

					// Determine remote path
					remotePath := strings.TrimSuffix(u.Path, "/") + "/" + filename

					log.WithFields(lf).WithFields(log.Fields{
						"name":        event.Name,
						"remote_path": remotePath,
					}).Debug("uploading to WebDAV")

					err = webdavClient.Upload(f, remotePath)
					if closeErr := f.Close(); closeErr != nil {
						log.WithFields(lf).Error("failed to close file: ", closeErr)
					}
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":        event.Name,
							"remote_path": remotePath,
						}).Error("failed to upload file to WebDAV: ", err)
						continue
					}

					log.WithFields(lf).WithFields(log.Fields{
						"name":        event.Name,
						"remote_path": remotePath,
					}).Info("successfully uploaded file to WebDAV")

					message := fmt.Sprintf("Uploaded %s to %s", filename, o.Destination)
					SendNotification("bucketsyncd", message)

				} else {
					// Handle S3 upload
					endpoint := u.Host
					tokens := strings.Split(u.Path, "/")
					const minTokens = 2
					if len(tokens) < minTokens {
						if closeErr := f.Close(); closeErr != nil {
							log.WithFields(lf).Error("failed to close file: ", closeErr)
						}
						log.WithFields(lf).Error("Invalid S3 path: ", u.Path)
						continue
					}
					awsBucket := tokens[1]
					awsFileKey := strings.Join(tokens[2:], "/") + "/" + filename
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"endpoint":   endpoint,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
					}).Debug("uploading to S3 bucket")

					// Determine remote to use to create a new MinIO client
					creds := credentials.Credentials{}
					credsFound := false
					configMutex.RLock()
					for _, remote := range config.Remotes {
						if remote.Endpoint == endpoint {
							creds = *credentials.NewStaticV4(remote.AccessKey, remote.SecretKey, "")
							credsFound = true
						}
					}
					configMutex.RUnlock()
					if !credsFound {
						if closeErr := f.Close(); closeErr != nil {
							log.WithFields(lf).Error("failed to close file: ", closeErr)
						}
						log.WithFields(lf).Error("No S3 credentials found for endpoint: ", endpoint)
						continue
					}
					mc, err := minio.New(endpoint, &minio.Options{
						Creds:  &creds,
						Secure: true,
					})
					if err != nil {
						if closeErr := f.Close(); closeErr != nil {
							log.WithFields(lf).Error("failed to close file: ", closeErr)
						}
						log.WithFields(lf).Error("failed to create MinIO client: ", err)
						continue
					}

					// Push object to S3 bucket
					fs, err := f.Stat()
					if err != nil {
						if closeErr := f.Close(); closeErr != nil {
							log.WithFields(lf).Error("failed to close file: ", closeErr)
						}
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
						}).Error("unable to query file size: ", err)
						continue
					}
					err = RetryOperation(func() error {
						ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
						defer cancel()
						_, err := mc.PutObject(ctx, awsBucket, awsFileKey, f, fs.Size(), minio.PutObjectOptions{})
						return err
					}, 3)
					if closeErr := f.Close(); closeErr != nil {
						log.WithFields(lf).Error("failed to close file: ", closeErr)
					}
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
						}).Error("failed to upload file to S3 after retries: ", err)
						continue
					}
					log.WithFields(lf).WithFields(log.Fields{
						"name":       event.Name,
						"awsBucket":  awsBucket,
						"awsFileKey": awsFileKey,
						"size":       fs.Size(),
					}).Info("uploaded to S3")

					message := fmt.Sprintf("Uploaded %s to %s", event.Name, o.Destination)
					SendNotification("bucketsyncd", message)
				}

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
		log.WithFields(lf).Error("failed to start watching folder: ", err)
		return
	}
}
