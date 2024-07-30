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
    Conn   *websocket.Conn `json:"-"`
    ID     int             `json:"id"`
    X int             `json:"x"`
    Y int             `json:"y"`
}

type Server struct {
    Clients map[int]*Client
    Mu      sync.Mutex
    NextID  int
}

func NewServer() *Server {
    return &Server{
        Clients: make(map[int]*Client),
        NextID:  1,
    }
}

func (s *Server) AddClient(conn *websocket.Conn) *Client {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    client := &Client{
        Conn:   conn,
        ID:     s.NextID,
        X: 0,
        Y: 0,
    }
    s.Clients[s.NextID] = client
    s.NextID++

    return client
}

func (s *Server) RemoveClient(id int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    delete(s.Clients, id)
}

func (s *Server) Broadcast() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    data, err := json.Marshal(s.Clients)
    if err != nil {
        log.Printf("Error marshalling data: %v", err)
        return
    }

    for _, client := range s.Clients {
        err := client.Conn.WriteMessage(websocket.TextMessage, data)
        if err != nil {
            log.Printf("Error writing message: %v", err)
        }
    }
}

func (s *Server) HandleClient(client *Client) {
    defer func() {
        s.RemoveClient(client.ID)
        client.Conn.Close()
        s.Broadcast()
    }()

    s.Broadcast()

    for {
        _, message, err := client.Conn.ReadMessage()
        if err != nil {
            log.Printf("Error reading message: %v", err)
            break
        }

        switch string(message) {
        case "x_plus":
            s.Mu.Lock()
            client.X++
            s.Mu.Unlock()
        case "x_minus":
            s.Mu.Lock()
            client.X--
            s.Mu.Unlock()
        case "y_plus":
            s.Mu.Lock()
            client.Y++
            s.Mu.Unlock()
        case "y_minus":
            s.Mu.Lock()
            client.Y--
            s.Mu.Unlock()
        default:
            log.Printf("Unknown command: %s", message)
        }

        s.Broadcast()
    }
}

func (s *Server) ServeWs(w http.ResponseWriter, r *http.Request) {
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

    client := s.AddClient(conn)
    go s.HandleClient(client)
}

func main() {
    server := NewServer()
    http.HandleFunc("/ws", server.ServeWs)

    fmt.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

