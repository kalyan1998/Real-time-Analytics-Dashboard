package main

import (
    "encoding/json"
    "log"
    "net/http"
    "strconv"
    "time"

    "github.com/gocql/gocql"
    "github.com/gorilla/websocket"
)

var clients = make(map[*websocket.Conn]bool)
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
var session *gocql.Session
var realTimeMode = true

type StockData struct {
    Symbol    string  `json:"symbol"`
    Price     float64 `json:"price"`
    Timestamp int64   `json:"timestamp"`
}

func initCassandra() {
    cluster := gocql.NewCluster("localhost")
    cluster.Keyspace = "stocks"
    cluster.Consistency = gocql.Quorum
    var err error
    session, err = cluster.CreateSession()  // Assign to the global session
    if err != nil {
        log.Fatalf("Could not connect to Cassandra: %v", err)
    }
    go streamData()
}


func streamData() {
    ticker := time.NewTicker(time.Second)
    for range ticker.C {
        if realTimeMode {
            sendLatestDataToClients()
        }
    }
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
    ws, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Fatal("Error upgrading to WebSocket:", err)
    }
    defer ws.Close()
    clients[ws] = true

    log.Println("WebSocket connection established")

    ws.SetCloseHandler(func(code int, text string) error {
        log.Printf("WebSocket closed with code %d and text: %s", code, text)
        delete(clients, ws)
        return nil
    })
}

func sendLatestDataToClients() {
    now := time.Now().Unix()
    iter := session.Query("SELECT symbol, price, timestamp FROM stock_prices WHERE timestamp > ? LIMIT 5", now-2).Iter()
    var sd StockData
    var count int
    for iter.Scan(&sd.Symbol, &sd.Price, &sd.Timestamp) {
        jsonData, _ := json.Marshal(sd)
        for client := range clients {
            if err := client.WriteMessage(websocket.TextMessage, jsonData); err != nil {
                log.Printf("Error sending message to client: %v", err)
                client.Close()
                delete(clients, client)
            } else {
                log.Printf("Sent data to client: %s", jsonData)
                count++
            }
        }
    }
    log.Printf("Total messages sent: %d", count)
    if err := iter.Close(); err != nil {
        log.Printf("Error querying Cassandra: %v", err)
    }
}

func handleData(w http.ResponseWriter, r *http.Request) {
    symbol := r.URL.Query().Get("symbol")
    from := r.URL.Query().Get("from")
    to := r.URL.Query().Get("to")
    fromTimestamp, _ := strconv.ParseInt(from, 10, 64)
    toTimestamp, _ := strconv.ParseInt(to, 10, 64)

    realTimeMode = false
    defer func() { realTimeMode = true }()

    query := "SELECT symbol, price, timestamp FROM stock_prices WHERE symbol = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp ASC"
    iter := session.Query(query, symbol, fromTimestamp, toTimestamp).Iter()
    var sd StockData
    results := make([]StockData, 0)
    for iter.Scan(&sd.Symbol, &sd.Price, &sd.Timestamp) {
        results = append(results, sd)
    }
    if err := iter.Close(); err != nil {
        log.Printf("Error querying Cassandra: %v", err)
        http.Error(w, "Error querying database", http.StatusInternalServerError)
        return
    }
    if len(results) == 0 {
        http.Error(w, "No data available for the selected range", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(results)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
    log.Println("Requested URL:", r.URL.Path)
    if r.URL.Path != "/" {
        http.Error(w, "404 not found.", http.StatusNotFound)
        return
    }
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    http.ServeFile(w, r, "./static/index.html")
}


func main() {
    initCassandra()
    defer session.Close()
    fs := http.FileServer(http.Dir("./static"))
    http.Handle("/static/", http.StripPrefix("/static/", fs))
    http.HandleFunc("/", serveHome)
    http.HandleFunc("/ws", handleConnections)
    http.HandleFunc("/data", handleData)

    log.Println("HTTP server started on :8080")
    http.ListenAndServe(":8080", nil)
}
