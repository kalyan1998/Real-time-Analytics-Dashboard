package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/segmentio/kafka-go"
)

type StockData struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

func main() {
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    "stocks",
		Balancer: &kafka.LeastBytes{},
	})

	symbols := []string{"AAPL", "GOOGL", "MSFT", "AMZN", "FB"}
	rand.Seed(time.Now().UnixNano())

	for {
		data := StockData{
			Symbol:    symbols[rand.Intn(len(symbols))],
			Price:     100 + rand.Float64()*(500-100),
			Timestamp: time.Now().Unix(),
		}
		dataBytes, _ := json.Marshal(data)
		err := w.WriteMessages(context.Background(), kafka.Message{
			Value: dataBytes,
		})
		if err != nil {
			panic(err)
		}
		time.Sleep(1 * time.Second)
	}
}

