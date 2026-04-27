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

	amqpConfig := amqp.Config{
		Properties: amqp.NewConnectionProperties(),
	}
	amqpConfig.Properties.SetClientConnectionName("bucketsyncd")
	conn, err := amqp.DialConfig(in.Source, amqpConfig)
	if err != nil {
		log.WithFields(lf).Error("failed to connect to AMQP service: ", err)
		return
	}
	connections = append(connections, conn)
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
		"bucketsyncd",
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
			var event S3Event
			err := json.Unmarshal(d.Body, &event)
			if err != nil {
				log.WithFields(lf).Error("failed to parse JSON payload: ", err)
				if nackErr := d.Nack(false, true); nackErr != nil { // Requeue for retry
					log.WithFields(lf).Error("failed to nack message: ", nackErr)
				}
				return
			}

			// Process each record in the event
			for _, record := range event.Records {
				bucketName := record.S3.Bucket.Name
				key, err := url.QueryUnescape(record.S3.Object.Key)
				if err != nil {
					log.WithFields(lf).Errorf("invalid URL-encoded key: %s", record.S3.Object.Key)
					if nackErr := d.Nack(false, false); nackErr != nil { // Don't requeue invalid messages
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					return
				}
				size := record.S3.Object.Size
				log.WithFields(lf).WithFields(log.Fields{
					"bucket": bucketName,
					"key":    key,
					"size":   size,
				}).Debugf("event '%s' received", event.EventName)

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
					return
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
					return
				}

				// Fetch given file from object storage
				opts := minio.GetObjectOptions{}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				obj, err := mc.GetObject(ctx, bucketName, key, opts)
				if err != nil {
					log.WithFields(lf).Error("failed to fetch object from MinIO: ", err)
					if nackErr := d.Nack(false, true); nackErr != nil {
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					return
				}
				defer func() {
					if err := obj.Close(); err != nil {
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
					return
				}
				defer func() {
					if err := localFile.Close(); err != nil {
						log.WithFields(lf).Error("failed to close local file: ", err)
					}
				}()

				stat, err := obj.Stat()
				if err != nil {
					log.WithFields(lf).Error("failed to get object stat: ", err)
					if nackErr := d.Nack(false, true); nackErr != nil {
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					return
				}

				if _, err := io.CopyN(localFile, obj, stat.Size); err != nil {
					log.WithFields(lf).Error("failed to copy file from reader: ", err)
					if nackErr := d.Nack(false, true); nackErr != nil {
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					return
				}

				log.WithFields(lf).WithFields(log.Fields{
					"filename": localFilename,
					"size":     size,
				}).Info("retrieved remote object to local file")
			}

			// Acknowledge queued message after successful processing
			err = d.Ack(false)
				if err != nil {
					log.WithFields(lf).Error("failed to copy file from reader: ", err)
					if nackErr := d.Nack(false, true); nackErr != nil {
						log.WithFields(lf).Error("failed to nack message: ", nackErr)
					}
					return
				}
		}
	}()
}

func inboundClose() {
	for _, c := range connections {
		if err := c.Close(); err != nil {
			log.Errorf("unable to close AMQP connection: %s", err)
		}
	}
}
