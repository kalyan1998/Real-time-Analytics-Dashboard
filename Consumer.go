package main

import (
    "context"
    "encoding/json"
    "log"

    "github.com/gocql/gocql"
    "github.com/segmentio/kafka-go"
)

func main() {
    // Setup Cassandra connection
    cluster := gocql.NewCluster("localhost")
    cluster.Keyspace = "stocks"
    cluster.Consistency = gocql.Quorum
    session, err := cluster.CreateSession()
    if err != nil {
        log.Fatalf("Could not connect to Cassandra: %v", err)
    }
    defer session.Close()

    // Ensure the table exists
    if err := ensureTableExists(session); err != nil {
        log.Fatalf("Could not ensure table existence: %v", err)
    }

    // Setup Kafka reader
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers:   []string{"localhost:9092"},
        Topic:     "stocks",
        Partition: 0,
        MinBytes:  10e3, // 10KB
        MaxBytes:  10e6, // 10MB
    })

    for {
        m, err := r.ReadMessage(context.Background())
        if err != nil {
            log.Fatalf("Could not read message: %v", err)
        }

        var msg map[string]interface{}
        if err := json.Unmarshal(m.Value, &msg); err != nil {
            log.Fatalf("Could not unmarshal message: %v", err)
        }

        symbol := msg["symbol"].(string)
        price := msg["price"].(float64)
        timestamp := int64(msg["timestamp"].(float64))

        // Insert data into Cassandra
        if err := session.Query(`INSERT INTO stock_prices (symbol, timestamp, price) VALUES (?, ?, ?)`,
            symbol, timestamp, price).Exec(); err != nil {
            log.Fatalf("Failed to insert data into Cassandra: %v", err)
        }

        log.Printf("Successfully inserted data into Cassandra: %v", msg)
    }
}

func ensureTableExists(session *gocql.Session) error {
    createTableCQL := `
    CREATE TABLE IF NOT EXISTS stock_prices (
        symbol text,
        timestamp bigint,
        price double,
        PRIMARY KEY (symbol, timestamp)
    ) WITH CLUSTERING ORDER BY (timestamp DESC)
    AND default_time_to_live = 3600;
    `
    return session.Query(createTableCQL).Exec()
}
