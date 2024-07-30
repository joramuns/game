package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "github.com/gorilla/websocket"
)

type Client struct {
    conn    *websocket.Conn `json:"-"`
    id      int             `json:"id"`
    number  int             `json:"number"`
}

type Server struct {
    clients map[int]*Client
    mu      sync.Mutex
    nextID  int
}

func NewServer() *Server {
    return &Server{
        clients: make(map[int]*Client),
        nextID:  1,
    }
}

func (s *Server) addClient(conn *websocket.Conn) *Client {
    s.mu.Lock()
    defer s.mu.Unlock()

    client := &Client{
        conn: conn,
        id:   s.nextID,
        number: 0,
    }
    s.clients[s.nextID] = client
    s.nextID++

    return client
}

func (s *Server) removeClient(id int) {
    s.mu.Lock()
    defer s.mu.Unlock()

    delete(s.clients, id)
}

func (s *Server) broadcast() {
    s.mu.Lock()
    defer s.mu.Unlock()

    data, err := json.Marshal(s.clients)
    if err != nil {
        log.Printf("Error marshalling data: %v", err)
        return
    }
    log.Printf("Json: %s", data)

    for _, client := range s.clients {
        err := client.conn.WriteMessage(websocket.TextMessage, data)
        if err != nil {
            log.Printf("Error writing message: %v", err)
        }
    }
}

func (s *Server) handleClient(client *Client) {
    defer func() {
        s.removeClient(client.id)
        client.conn.Close()
        s.broadcast()
    }()

    s.broadcast()

    for {
        _, message, err := client.conn.ReadMessage()
        if err != nil {
            log.Printf("Error reading message: %v", err)
            break
        }

        switch string(message) {
        case "increment":
            s.mu.Lock()
            client.number++
            s.mu.Unlock()
        case "decrement":
            s.mu.Lock()
            client.number--
            s.mu.Unlock()
        default:
            log.Printf("Unknown command: %s", message)
        }

        s.broadcast()
    }
}

func (s *Server) serveWs(w http.ResponseWriter, r *http.Request) {
    upgrader := websocket.Upgrader{
        CheckOrigin: func(r *http.Request) bool {
            return true
        },
    }

    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("Error upgrading connection: %v", err)
        return
    }

    client := s.addClient(conn)
    go s.handleClient(client)
}

func main() {
    server := NewServer()
    http.HandleFunc("/ws", server.serveWs)

    fmt.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

