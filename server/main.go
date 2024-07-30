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
    Conn *websocket.Conn `json:"-"`
    ID   int             `json:"id"`
    X    int             `json:"x"`
    Y    int             `json:"y"`
}

type Server struct {
    Clients map[int]*Client
    Grid    map[int]map[int]*Client // Grid[y][x] -> Client
    Mu      sync.Mutex
    NextID  int
}

func NewServer() *Server {
    return &Server{
        Clients: make(map[int]*Client),
        Grid:    make(map[int]map[int]*Client),
        NextID:  1,
    }
}

func (s *Server) AddClient(conn *websocket.Conn) *Client {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    client := &Client{
        Conn: conn,
        ID:   s.NextID,
        X:    0,
        Y:    0,
    }
    s.Clients[s.NextID] = client
    if s.Grid[client.Y] == nil {
        s.Grid[client.Y] = make(map[int]*Client)
    }
    s.Grid[client.Y][client.X] = client
    s.NextID++

    return client
}

func (s *Server) RemoveClient(id int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    client := s.Clients[id]
    if client != nil {
        delete(s.Grid[client.Y], client.X)
        if len(s.Grid[client.Y]) == 0 {
            delete(s.Grid, client.Y)
        }
        delete(s.Clients, id)
    }
}

func (s *Server) UpdateClientPosition(client *Client, newX, newY int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()
    if s.Grid[newY] != nil && s.Grid[newY][newX] != nil {
      return
    }

    // Remove client from old position
    delete(s.Grid[client.Y], client.X)
    if len(s.Grid[client.Y]) == 0 {
        delete(s.Grid, client.Y)
    }

    // Update client position
    client.X = newX
    client.Y = newY

    // Add client to new position
    if s.Grid[newY] == nil {
        s.Grid[newY] = make(map[int]*Client)
    }
    s.Grid[newY][newX] = client
}

func (s *Server) GetClientsInRange(x, y, rangeVal int) map[int]*Client {
    clientsInRange := make(map[int]*Client)
    for dy := -rangeVal; dy <= rangeVal; dy++ {
        for dx := -rangeVal; dx <= rangeVal; dx++ {
            nx, ny := x+dx, y+dy
            if row, exists := s.Grid[ny]; exists {
                if client, exists := row[nx]; exists {
                    clientsInRange[client.ID] = client
                }
            }
        }
    }
    return clientsInRange
}

func (s *Server) Broadcast() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    for _, client := range s.Clients {
        nearbyClients := s.GetClientsInRange(client.X, client.Y, 3)
        data, err := json.Marshal(nearbyClients)
        if err != nil {
            log.Printf("Error marshalling data: %v", err)
            continue
        }

        err = client.Conn.WriteMessage(websocket.TextMessage, data)
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

    // Send initial ID to client
    idData := map[string]int{"client_id": client.ID}
    idJSON, err := json.Marshal(idData)
    if err != nil {
        log.Printf("Error marshalling ID data: %v", err)
        return
    }
    if err := client.Conn.WriteMessage(websocket.TextMessage, idJSON); err != nil {
        log.Printf("Error sending ID to client: %v", err)
        return
    }

    s.Broadcast()

    for {
        _, message, err := client.Conn.ReadMessage()
        if err != nil {
            log.Printf("Error reading message: %v", err)
            break
        }

        switch string(message) {
        case "x_plus":
            s.UpdateClientPosition(client, client.X+1, client.Y)
        case "x_minus":
            s.UpdateClientPosition(client, client.X-1, client.Y)
        case "y_plus":
            s.UpdateClientPosition(client, client.X, client.Y+1)
        case "y_minus":
            s.UpdateClientPosition(client, client.X, client.Y-1)
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

