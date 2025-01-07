package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"os"
	"path/filepath"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/fsnotify/fsnotify"

	"github.com/ryanuber/go-glob"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	ctx := context.TODO()

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
			log.WithFields(lf).Error(err)
			return
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

					// Determine remote bucket details
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
					fs, _ := f.Stat()
					if err != nil {
						log.WithFields(lf).WithFields(log.Fields{
							"name":       event.Name,
							"awsBucket":  awsBucket,
							"awsFileKey": awsFileKey,
						}).Error("unable to query file size: ", err)
						return
					}
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

	// Set up watcher for each inbound source
	for i := 0; i < len(config.Inbound); i++ {
		in := config.Inbound[i]
		lf := log.Fields{
			"workflow": in.Name,
		}
		u, err := url.Parse(in.Source)
		if err != nil {
			log.WithFields(lf).Error("failed to parse AMQP connection string: ", err)
			return
		}
		lf = log.Fields{
			"workflow": in.Name,
			"source":   u.Redacted(),
			"exchange": in.Exchange,
			"queue":    in.Queue,
		}
		log.WithFields(lf).Info("configuring AMQP client for '", in.Description, "'")

		amqpConfig := amqp.Config{
			Properties: amqp.NewConnectionProperties(),
		}
		amqpConfig.Properties.SetClientConnectionName("bucketsyncd")
		conn, err := amqp.DialConfig(in.Source, amqpConfig)
		if err != nil {
			log.WithFields(lf).Error("failed to connect to AMQP service: ", err)
			return
		}
		go func() {
			log.WithFields(lf).Debugf("closing connection to AMQP service: %s", <-conn.NotifyClose(make(chan *amqp.Error)))
		}()

		// Bind to message queue
		channel, err := conn.Channel()
		if err != nil {
			log.WithFields(lf).Error("failed to declare AMQP channel: ", err)
			return
		}
		err = channel.QueueBind(
			in.Queue,
			in.Exchange,
			in.Exchange,
			false,
			nil,
		)
		if err != nil {
			log.WithFields(lf).Error("failed to bind to AMQP queue: ", err)
			return
		}
		log.WithFields(lf).Debug("queue bound to exchange")

		// Consume messages
		deliveries, err := channel.Consume(
			in.Queue,
			"backupsyncd",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.WithFields(lf).Error("failed to consume messages from AMQP queue: ", err)
			return
		}

		go func() {
			for d := range deliveries {
				log.WithFields(lf).Debugf(
					"got %dB delivery: [%v] %q",
					len(d.Body),
					d.DeliveryTag,
					d.Body,
				)

				// Parse JSON payload
				var message map[string]interface{}
				err := json.Unmarshal(d.Body, &message)
				if err != nil {
					log.WithFields(lf).Error("failed to parse JSON payload: ", err)
					return
				}
				eventName := message["EventName"].(string)
				records := message["Records"].([]interface{})
				for _, record := range records {
					// Extract details from record
					r := record.(map[string]interface{})
					s3 := r["s3"].(map[string]interface{})
					bucket := s3["bucket"].(map[string]interface{})
					bucketName := bucket["name"].(string)
					obj := s3["object"].(map[string]interface{})
					key, err := url.QueryUnescape(obj["key"].(string))
					if err != nil {
						log.WithFields(lf).Errorf("invalid URL-encoded key: %s", obj["key"])
						return
					}
					size := obj["size"].(float64)
					log.WithFields(lf).WithFields(log.Fields{
						"bucket": bucketName,
						"key":    key,
						"size":   size,
					}).Debugf("event '%s' received", eventName)

					// // Format record as JSON [DEBUG]
					// jsonMessage, err := json.Marshal(record)
					// if err != nil {
					// 	log.WithFields(lf).Error("failed to format message as JSON: ", err)
					// 	return
					// }
					// fmt.Println(jsonMessage)

					// Determine remote to use to create a new MinIO client
					creds := credentials.Credentials{}
					credsFound := false
					var remote Remote
					for _, remote = range config.Remotes {
						if remote.Name == in.Remote {
							creds = *credentials.NewStaticV4(remote.AccessKey, remote.SecretKey, "")
							credsFound = true
							break
						}
					}
					if !credsFound {
						log.WithFields(lf).Error("no credentials found")
						return
					}
					log.WithFields(lf).Debugf("connecting to endpoint '%s'", remote.Endpoint)
					mc, err := minio.New(remote.Endpoint, &minio.Options{
						Creds:  &creds,
						Secure: true,
					})
					if err != nil {
						log.WithFields(lf).Error("failed to create MinIO client: ", err)
						return
					}

					// Fetch given file from object storage
					opts := minio.GetObjectOptions{}
					reader, err := mc.GetObject(ctx, bucketName, key, opts)
					if err != nil {
						log.WithFields(lf).Error("failed to fetch object from MinIO: ", err)
						return
					}
					defer reader.Close()

					localFilename := fmt.Sprintf("%s/%s", in.Destination, filepath.Base(key))
					localFile, err := os.OpenFile(localFilename, os.O_RDWR|os.O_CREATE, 0644)
					if err != nil {
						log.WithFields(lf).Error("failed to create local file: ", err)
						return
					}
					defer localFile.Close()

					stat, err := reader.Stat()
					if err != nil {
						log.WithFields(lf).Error("failed to get reader size: ", err)
						return
					}

					if _, err := io.CopyN(localFile, reader, stat.Size); err != nil {
						log.WithFields(lf).Error("failed to copy file from reader: ", err)
						return
					}

					log.WithFields(lf).WithFields(log.Fields{
						"filename": localFilename,
						"size":     size,
					}).Info("retrieved remote object to local file")
				}

				// Acknowledge queued message
				err = d.Ack(false)
				if err != nil {
					log.WithFields(lf).Error("failed to acknowledge AMQP message: ", err)
				}
			}
		}()
	}

	<-done
}
