package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	"os"
	"path/filepath"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var connections []*amqp.Connection

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
				ctx := context.TODO()
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

func inboundClose() {
	for _, c := range connections {
		if err := c.Close(); err != nil {
			log.Errorf("unable to close AMQP connection: %s", err)
		}
	}
}
