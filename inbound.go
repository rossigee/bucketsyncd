package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"os"
	"path/filepath"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Event represents the structure of an S3 event notification
type S3Event struct {
	EventName string      `json:"EventName"`
	Records   []S3Record  `json:"Records"`
}

type S3Record struct {
	S3 S3Info `json:"s3"`
}

type S3Info struct {
	Bucket BucketInfo `json:"bucket"`
	Object ObjectInfo `json:"object"`
}

type BucketInfo struct {
	Name string `json:"name"`
}

type ObjectInfo struct {
	Key  string  `json:"key"`
	Size float64 `json:"size"`
}



var connections []*amqp.Connection

// nolint:gocognit,funlen // This function handles the main AMQP processing logic
func inbound(in Inbound) {
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

	// Reconnection loop
	for attempt := 0; ; attempt++ {
		amqpConfig := amqp.Config{
			Properties: amqp.NewConnectionProperties(),
		}
		amqpConfig.Properties.SetClientConnectionName("bucketsyncd")
		conn, err := amqp.DialConfig(in.Source, amqpConfig)
		if err != nil {
			backoffSeconds := 1 << uint(attempt) // Exponential backoff
			if backoffSeconds > 300 {              // Cap at 5 minutes
				backoffSeconds = 300
			}
			log.WithFields(lf).WithFields(log.Fields{
				"attempt": attempt + 1,
				"backoff": backoffSeconds,
				"error":   err,
			}).Error("failed to connect to AMQP service, retrying")
			time.Sleep(time.Duration(backoffSeconds) * time.Second)
			continue
		}

		log.WithFields(lf).Info("successfully connected to AMQP service")
		connections = append(connections, conn)

		// Reset attempt counter on successful connection
		attempt = 0

		// Channel for connection close notifications
		connCloseChan := make(chan *amqp.Error)
		conn.NotifyClose(connCloseChan)

		// Bind to message queue
		channel, err := conn.Channel()
		if err != nil {
			log.WithFields(lf).Error("failed to declare AMQP channel: ", err)
			conn.Close()
			continue
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
			conn.Close()
			continue
		}
		log.WithFields(lf).Debug("queue bound to exchange")

		// Consume messages
		deliveries, err := channel.Consume(
			in.Queue,
			"bucketsyncd",
			false,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.WithFields(lf).Error("failed to consume messages from AMQP queue: ", err)
			conn.Close()
			continue
		}

		log.WithFields(lf).Info("AMQP consumer started, processing messages")

		// Message processing loop
		for {
			select {
			case d, ok := <-deliveries:
				if !ok {
					log.WithFields(lf).Warn("deliveries channel closed")
					// Clean up connection and continue to reconnect
					if conn != nil && !conn.IsClosed() {
						conn.Close()
					}
					log.WithFields(lf).Info("reconnecting to AMQP service in 5 seconds")
					time.Sleep(5 * time.Second)
					continue
				}

				// Parse JSON payload
				var s3Event S3Event
				var err error
				err = json.Unmarshal(d.Body, &s3Event)
				if err != nil {
					log.WithFields(lf).Error("failed to parse JSON payload: ", err)
					if nackErr := d.Nack(false, true); nackErr != nil { // Requeue for retry
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					continue
				}

				// Process each record in the event
				for _, record := range s3Event.Records {
					bucketName := record.S3.Bucket.Name
					key, err := url.QueryUnescape(record.S3.Object.Key)
					if err != nil {
						log.WithFields(lf).Errorf("invalid URL-encoded key: %s", record.S3.Object.Key)
						if nackErr := d.Nack(false, false); nackErr != nil { // Don't requeue invalid messages
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}
					size := record.S3.Object.Size
					log.WithFields(lf).WithFields(log.Fields{
						"bucket": bucketName,
						"key":    key,
						"size":   size,
					}).Debugf("event '%s' received", s3Event.EventName)

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
						if nackErr := d.Nack(false, true); nackErr != nil { // Requeue, maybe config issue
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}
					log.WithFields(lf).Debugf("connecting to endpoint '%s'", remote.Endpoint)
					mc, err := minio.New(remote.Endpoint, &minio.Options{
						Creds:  &creds,
						Secure: true,
					})
					if err != nil {
						log.WithFields(lf).Error("failed to create MinIO client: ", err)
						if nackErr := d.Nack(false, true); nackErr != nil {
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}

					// Fetch given file from object storage
					opts := minio.GetObjectOptions{}
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()
					minioObj, err := mc.GetObject(ctx, bucketName, key, opts)
					if err != nil {
						log.WithFields(lf).Error("failed to fetch object from MinIO: ", err)
						if nackErr := d.Nack(false, true); nackErr != nil {
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}
					defer func() {
						if err := minioObj.Close(); err != nil {
							log.WithFields(lf).Error("failed to close object: ", err)
						}
					}()

					localFilename := fmt.Sprintf("%s/%s", in.Destination, filepath.Base(key))
					const filePerms = 0600
					// #nosec G304 - This is intentional file creation in configured destination
					localFile, err := os.OpenFile(localFilename, os.O_RDWR|os.O_CREATE, filePerms)
					if err != nil {
						log.WithFields(lf).Error("failed to create local file: ", err)
						if nackErr := d.Nack(false, true); nackErr != nil {
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}
					defer func() {
						if err := localFile.Close(); err != nil {
							log.WithFields(lf).Error("failed to close local file: ", err)
						}
					}()

					stat, err := minioObj.Stat()
					if err != nil {
						log.WithFields(lf).Error("failed to get object stat: ", err)
						if nackErr := d.Nack(false, true); nackErr != nil {
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}

					if _, err := io.CopyN(localFile, minioObj, stat.Size); err != nil {
						log.WithFields(lf).Error("failed to copy file from reader: ", err)
						if nackErr := d.Nack(false, true); nackErr != nil {
							log.WithFields(lf).Error("failed to nack message: ", nackErr)
						}
						continue
					}

					log.WithFields(lf).WithFields(log.Fields{
						"filename": localFilename,
						"size":     size,
					}).Info("retrieved remote object to local file")

					// Acknowledge queued message after successful processing
					err = d.Ack(false)
					if err != nil {
						log.WithFields(lf).Error("failed to acknowledge AMQP message: ", err)
					}
				}

			case connErr, ok := <-connCloseChan:
				if !ok {
					log.WithFields(lf).Warn("connection close channel closed")
				} else {
					log.WithFields(lf).WithFields(log.Fields{
						"error": connErr,
					}).Warn("AMQP connection closed, attempting reconnection")
				}
				// Clean up connection and continue to reconnect
				if conn != nil && !conn.IsClosed() {
					conn.Close()
				}
				log.WithFields(lf).Info("reconnecting to AMQP service in 5 seconds")
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}
}

func inboundClose() {
	for _, c := range connections {
		if err := c.Close(); err != nil {
			log.Errorf("unable to close AMQP connection: %s", err)
		}
	}
}
