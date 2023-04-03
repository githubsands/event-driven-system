package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/segmentio/kafka-go"
)

func getBatchSize() int {
	value := os.Getenv("BATCH_SIZE")
	if value == "" {
		log.Fatalf("user must set BATCH_SIZE")
	}
	val, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("Failed to convert BATCH_SIZE to int")
	}
	return val
}

func main() {
	batchSize := getBatchSize()
	var db *sql.DB
	var err error
SQL_RETRY:
	for {
		db, err = sql.Open("mysql", os.Getenv("POSTGRES_STRING_CLUSTER"))
		if err == nil {
			break SQL_RETRY
		}
	}
	defer db.Close()
	kafkaURI := os.Getenv("KAFKA_CLUSTER_URI")
	if kafkaURI == "" {
		log.Fatalf("user must set KAFKA_URI")
	}
	brokers := []string{kafkaURI}
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:   brokers,
		Topic:     "u-transactions",
		Balancer:  &kafka.LeastBytes{},
		BatchSize: batchSize,
	})
	query := "SELECT tx_bytes FROM transactions;"
	var rows *sql.Rows
	for {
	DB_QUERY:
		for {
			rows, err = db.Query(query)
			if err == nil {
				break DB_QUERY
			}
			fmt.Println("Error querying DB: ", err)
		}
		for rows.Next() {
			var value string
			err := rows.Scan(&value)
			if err != nil {
				log.Printf("Failed to scan row: %v", err)
				continue
			}
			fmt.Println("Writing message: ", value)
			err = writer.WriteMessages(nil, kafka.Message{
				Value: []byte(value),
			})
			if err != nil {
				log.Printf("Failed to write message to Kafka: %v", err)
			} else {
				fmt.Printf("Sent message to Kafka: %s\n", value)
			}
		}
		rows.Close()
		time.Sleep(1 * time.Minute)
	}
}
