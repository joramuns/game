package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "math/rand"

    "github.com/go-redis/redis/v8"
    "github.com/gorilla/websocket"
    "server/objects"
)

var ctx = context.Background()

type Server struct {
    Clients map[int]*objects.Client
    Grid    map[int]map[int]*objects.Client // Grid[y][x] -> Client
    Mu      sync.Mutex
    NextID  int
    RedisClient *redis.Client
}

func NewServer(redisClient *redis.Client) *Server {
    return &Server{
        Clients: make(map[int]*objects.Client),
        Grid:    make(map[int]map[int]*objects.Client),
        NextID:  1,
        RedisClient: redisClient,
    }
}

func (s *Server) AddClient(conn *websocket.Conn) *objects.Client {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    client := &objects.Client{
        Conn: conn,
        ID:   s.NextID,
        X:    0,
        Y:    0,
    }
    s.Clients[s.NextID] = client
    if s.Grid[client.Y] == nil {
        s.Grid[client.Y] = make(map[int]*objects.Client)
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

func (s *Server) UpdateClientPosition(client *objects.Client, newX, newY int) {
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
        s.Grid[newY] = make(map[int]*objects.Client)
    }
    s.Grid[newY][newX] = client
    // Обновляем данные клиента в Redis

    data, _ := json.Marshal(client)
    s.RedisClient.Set(ctx, fmt.Sprintf("client:%d", client.ID), data, 0)
}

func (s *Server) GetClientsInRange(x, y, rangeVal int) map[int]*objects.Client {
    clientsInRange := make(map[int]*objects.Client)
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
      if client.Conn == nil {
        continue
      }
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

func (s *Server) HandleClient(client *objects.Client) {
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

func (s *Server) GenerateFakeClients(num int, size_x int, size_y int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    for i := 0; i < num; i++ {
        x := rand.Intn(size_x)
        y := rand.Intn(size_y)
        client := &objects.Client{
            ID:     s.NextID,
            X:      x,
            Y:      y,
        }
        s.Clients[s.NextID] = client
        if s.Grid[client.Y] == nil {
            s.Grid[client.Y] = make(map[int]*objects.Client)
        }
        s.Grid[client.Y][client.X] = client
        s.NextID++

        // Сохраняем фейкового клиента в Redis
        data, _ := json.Marshal(client)
        s.RedisClient.Set(ctx, fmt.Sprintf("client:%d", client.ID), data, 0)
    }
}

func (s *Server) SaveMapToRedis() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    // Сохраняем состояние карты в Redis
    mapData, _ := json.Marshal(s.Grid)
    s.RedisClient.Set(ctx, "map:state", mapData, 0)
    log.Println("Map saved")
}

func (s *Server) LoadMapFromRedis() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    // Загружаем состояние карты из Redis
    mapData, err := s.RedisClient.Get(ctx, "map:state").Result()
    if err == redis.Nil {
        log.Println("No map state found in Redis")
        return
    } else if err != nil {
        log.Fatalf("Error loading map state from Redis: %v", err)
    }

    var loadedGrid map[int]map[int]*objects.Client
    if err := json.Unmarshal([]byte(mapData), &loadedGrid); err != nil {
        log.Fatalf("Error unmarshalling map state from Redis: %v", err)
    }

    s.Grid = loadedGrid

    // Восстанавливаем клиентов из карты
    for _, row := range s.Grid {
        for _, client := range row {
            s.Clients[client.ID] = client
        }
    }

    // Обновляем следующий ID
    if len(s.Clients) > 0 {
        s.NextID = len(s.Clients) + 1
    }
}

func main() {
    // Инициализируем подключение к Redis
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379", // Адрес Redis сервера
    })

    server := NewServer(rdb)

    // Загружаем состояние карты из Redis
    server.LoadMapFromRedis()

    // Генерируем псевдоклиентов при запуске
    if len(server.Clients) == 0 {
        server.GenerateFakeClients(10, 10, 10)
    }

    // Сохраняем состояние карты в Redis
    defer server.SaveMapToRedis()

    http.HandleFunc("/ws", server.ServeWs)

    fmt.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

