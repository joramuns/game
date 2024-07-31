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
    Objects map[int]*objects.Object
    Gred    map[int]map[int]*objects.Object // Grid[y][x] -> Object
    Mu      sync.Mutex
    NextID  int
    RedisClient *redis.Client
}

func NewServer(redisClient *redis.Client) *Server {
    return &Server{
        Clients: make(map[int]*objects.Client),
        Objects: make(map[int]*objects.Object),
        Gred:    make(map[int]map[int]*objects.Object),
        NextID:  1,
        RedisClient: redisClient,
    }
}

func (s *Server) AddClient(conn *websocket.Conn) *objects.Client {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    object := &objects.Object{
      ID: s.NextID,
      X: 0,
      Y: 0,
    }
    s.Objects[s.NextID] = object

    client := &objects.Client{
        Conn: conn,
        Player: object,
        ID:   s.NextID,
    }
    s.Clients[s.NextID] = client

    if s.Gred[object.Y] == nil {
        s.Gred[object.Y] = make(map[int]*objects.Object)
    }
    s.Gred[object.Y][object.X] = client.Player

    s.NextID++

    return client
}

func (s *Server) RemoveObject(id int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    object := s.Objects[id]
    if object != nil {
      delete(s.Gred[object.Y], object.X)
      if len(s.Gred[object.Y]) == 0 {
          delete(s.Gred, object.Y)
      }
      delete(s.Objects, id)
    }
}

func (s *Server) RemoveClient(id int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    client := s.Clients[id]
    if client != nil {
        s.RemoveObject(client.Player.ID)
        delete(s.Clients, id)
    }
}

func (s *Server) UpdateObjectPosition(object *objects.Object, newX, newY int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()
    if s.Gred[newY] != nil && s.Gred[newY][newX] != nil {
      return
    }

    // Remove object from old position
    delete(s.Gred[object.Y], object.X)
    if len(s.Gred[object.Y]) == 0 {
        delete(s.Gred, object.Y)
    }

    // Update object position
    object.X = newX
    object.Y = newY

    // Add object to new position
    if s.Gred[newY] == nil {
        s.Gred[newY] = make(map[int]*objects.Object)
    }
    s.Gred[newY][newX] = object

    // Update object data in Redis
    data, _ := json.Marshal(object)
    s.RedisClient.Set(ctx, fmt.Sprintf("client:%d", object.ID), data, 0)
}

func (s *Server) GetObjectsInRange(x, y, rangeVal int) map[int]*objects.Object {
    objectsInRange := make(map[int]*objects.Object)
    for dy := -rangeVal; dy <= rangeVal; dy++ {
        for dx := -rangeVal; dx <= rangeVal; dx++ {
            nx, ny := x+dx, y+dy
            if row, exists := s.Gred[ny]; exists {
                if object, exists := row[nx]; exists {
                    objectsInRange[object.ID] = object
                }
            }
        }
    }
    return objectsInRange
}

func (s *Server) Broadcast() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    for _, client := range s.Clients {
      if client.Conn == nil {
        continue
      }
        nearbyObjects := s.GetObjectsInRange(client.Player.X, client.Player.Y, 3)
        data, err := json.Marshal(nearbyObjects)
        if err != nil {
            log.Printf("Error marshalling data: %v", err)
            continue
        }
        log.Printf("JSON %s", data)

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

    // Send initial associated player object ID to client
    idData := map[string]int{"client_id": client.Player.ID}
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
            s.UpdateObjectPosition(client.Player, client.Player.X+1, client.Player.Y)
        case "x_minus":
            s.UpdateObjectPosition(client.Player, client.Player.X-1, client.Player.Y)
        case "y_plus":
            s.UpdateObjectPosition(client.Player, client.Player.X, client.Player.Y+1)
        case "y_minus":
            s.UpdateObjectPosition(client.Player, client.Player.X, client.Player.Y-1)
          case "exit":
            log.Printf("Server terminated")
            return
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

func (s *Server) GenerateObjects(num int, size_x int, size_y int) {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    for i := 0; i < num; i++ {
        x := rand.Intn(size_x)
        y := rand.Intn(size_y)
        object := &objects.Object{
            ID:     s.NextID,
            X:      x,
            Y:      y,
        }
        s.Objects[s.NextID] = object
        if s.Gred[object.Y] == nil {
            s.Gred[object.Y] = make(map[int]*objects.Object)
        }
        s.Gred[object.Y][object.X] = object
        s.NextID++

        // Сохраняем фейкового клиента в Redis
        data, _ := json.Marshal(object)
        s.RedisClient.Set(ctx, fmt.Sprintf("client:%d", object.ID), data, 0)
    }
}

func (s *Server) SaveMapToRedis() {
    s.Mu.Lock()
    defer s.Mu.Unlock()

    // Сохраняем состояние карты в Redis
    mapData, _ := json.Marshal(s.Gred)
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

    var loadedGrid map[int]map[int]*objects.Object
    if err := json.Unmarshal([]byte(mapData), &loadedGrid); err != nil {
        log.Fatalf("Error unmarshalling map state from Redis: %v", err)
    }

    s.Gred = loadedGrid

    // Recover objects from map
    for _, row := range s.Gred {
        for _, object := range row {
            s.Objects[object.ID] = object
        }
    }

    // Обновляем следующий ID
    if len(s.Objects) > 0 {
        s.NextID = len(s.Objects) + 1
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
    if len(server.Objects) == 0 {
        server.GenerateObjects(10, 10, 10)
    }

    // Сохраняем состояние карты в Redis
    defer server.SaveMapToRedis()

    http.HandleFunc("/ws", server.ServeWs)

    fmt.Println("Server started on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

