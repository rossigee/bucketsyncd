package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var connections []*amqp.Connection

// Helper function for exponential backoff
func retryWithBackoff(attempts int, operation func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = operation()
		if err == nil {
			return nil
		}
		log.WithFields(log.Fields{"attempt": i + 1}).Warnf("Operation failed: %v, retrying...", err)
		sleep := time.Duration(1<<uint(i)) * time.Second
		jitter := time.Duration(rand.Intn(100)) * time.Millisecond
		time.Sleep(sleep + jitter)
	}
	return fmt.Errorf("operation failed after %d attempts: %w", attempts, err)
}

// consumeMessages processes messages from the deliveries channel
func consumeMessages(ctx context.Context, deliveries <-chan amqp.Delivery, in Inbound, lf log.Fields) {
	for {
		select {
		case <-ctx.Done():
			log.WithFields(lf).Info("stopping message consumption")
			return
		case d, ok := <-deliveries:
			if !ok {
				log.WithFields(lf).Warn("deliveries channel closed")
				return
			}
			log.WithFields(lf).Debugf(
				"got %dB delivery: [%v] %q",
				len(d.Body),
				d.DeliveryTag,
				d.Body,
			)

			// Parse JSON payload
			var message map[string]interface{}
			if err := json.Unmarshal(d.Body, &message); err != nil {
				log.WithFields(lf).Error("failed to parse JSON payload: ", err)
				continue // Skip to next message
			}
			eventName, _ := message["EventName"].(string)
			records, _ := message["Records"].([]interface{})
			for _, record := range records {
				// Extract details from record
				r, _ := record.(map[string]interface{})
				s3, _ := r["s3"].(map[string]interface{})
				bucket, _ := s3["bucket"].(map[string]interface{})
				bucketName, _ := bucket["name"].(string)
				obj, _ := s3["object"].(map[string]interface{})
				key, err := url.QueryUnescape(obj["key"].(string))
				if err != nil {
					log.WithFields(lf).Errorf("invalid URL-encoded key: %s", obj["key"])
					continue
				}
				size, _ := obj["size"].(float64)
				log.WithFields(lf).WithFields(log.Fields{
					"bucket": bucketName,
					"key":    key,
					"size":   size,
				}).Debugf("event '%s' received", eventName)

				// Initialize MinIO client with retries
				var mc *minio.Client
				err = retryWithBackoff(5, func() error {
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
						return fmt.Errorf("no credentials found")
					}
					log.WithFields(lf).Debugf("connecting to endpoint '%s'", remote.Endpoint)
					mc, err = minio.New(remote.Endpoint, &minio.Options{
						Creds:  &creds,
						Secure: true,
					})
					return err
				})
				if err != nil {
					log.WithFields(lf).Error("failed to create MinIO client after retries: ", err)
					continue
				}

				// Fetch given file from object storage
				opts := minio.GetObjectOptions{}
				reader, err := mc.GetObject(ctx, bucketName, key, opts)
				if err != nil {
					log.WithFields(lf).Error("failed to fetch object from MinIO: ", err)
					continue
				}
				defer reader.Close()

				localFilename := fmt.Sprintf("%s/%s", in.Destination, filepath.Base(key))
				localFile, err := os.OpenFile(localFilename, os.O_RDWR|os.O_CREATE, 0644)
				if err != nil {
					log.WithFields(lf).Error("failed to create local file: ", err)
					continue
				}
				defer localFile.Close()

				stat, err := reader.Stat()
				if err != nil {
					log.WithFields(lf).Error("failed to get reader size: ", err)
					continue
				}

				if _, err := io.CopyN(localFile, reader, stat.Size); err != nil {
					log.WithFields(lf).Error("failed to copy file from reader: ", err)
					continue
				}

				log.WithFields(lf).WithFields(log.Fields{
					"filename": localFilename,
					"size":     size,
				}).Info("retrieved remote object to local file")
			}

			// Acknowledge queued message
			if err := d.Ack(false); err != nil {
				log.WithFields(lf).Error("failed to acknowledge AMQP message: ", err)
			}
		}
	}
}

func inbound(ctx context.Context, in Inbound) {
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

	var conn *amqp.Connection
	amqpConfig := amqp.Config{
		Properties: amqp.NewConnectionProperties(),
	}
	amqpConfig.Properties.SetClientConnectionName("bucketsyncd")

	err = retryWithBackoff(5, func() error {
		conn, err = amqp.DialConfig(in.Source, amqpConfig)
		return err
	})
	if err != nil {
		log.WithFields(lf).Error("failed to connect to AMQP service after retries: ", err)
		return
	}
	connections = append(connections, conn)

	// Setup channel and bind queue
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

	// Start initial message consumption
	go consumeMessages(ctx, deliveries, in, lf)

	// Monitor connection and reconnect
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.WithFields(lf).Info("shutting down AMQP connection")
				if err := conn.Close(); err != nil {
					log.WithFields(lf).Error("failed to close AMQP connection: ", err)
				}
				return
			case err := <-conn.NotifyClose(make(chan *amqp.Error)):
				log.WithFields(lf).Warnf("AMQP connection closed: %v", err)

				// Attempt to reconnect
				amqperr := retryWithBackoff(5, func() error {
					newConn, err := amqp.DialConfig(in.Source, amqpConfig)
					if err == nil {
						conn = newConn
						connections = append(connections, newConn)
						return nil
					}
					return err
				})
				if amqperr != nil {
					log.WithFields(lf).Error("failed to reconnect to AMQP service: ", amqperr)
					return
				}

				// Rebind queue and resume consuming
				channel, amqperr := conn.Channel()
				if amqperr != nil {
					log.WithFields(lf).Error("failed to declare AMQP channel after reconnect: ", amqperr)
					return
				}
				amqperr = channel.QueueBind(
					in.Queue,
					in.Exchange,
					in.Exchange,
					false,
					nil,
				)
				if amqperr != nil {
					log.WithFields(lf).Error("failed to bind to AMQP queue after reconnect: ", amqperr)
					return
				}
				deliveries, amqperr := channel.Consume(
					in.Queue,
					"backupsyncd",
					false,
					false,
					false,
					false,
					nil,
				)
				if amqperr != nil {
					log.WithFields(lf).Error("failed to consume messages after reconnect: ", amqperr)
					return
				}

				// Start consuming messages on new deliveries channel
				go consumeMessages(ctx, deliveries, in, lf)
			}
		}
	}()
}

func inboundClose() {
	for i, c := range connections {
		if c == nil || c.IsClosed() {
			continue
		}
		if err := c.Close(); err != nil {
			log.Errorf("unable to close AMQP connection %d: %s", i, err)
		} else {
			log.Debugf("closed AMQP connection %d", i)
		}
	}
	connections = nil
}
