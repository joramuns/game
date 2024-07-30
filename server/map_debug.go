package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/go-redis/redis/v8"
    "server/objects"
)

var ctx = context.Background()

func main() {
    // Инициализируем подключение к Redis
    rdb := redis.NewClient(&redis.Options{
        Addr: "localhost:6379", // Адрес Redis сервера
    })

    // Получаем все ключи, которые начинаются с "client:"
    keys, err := rdb.Keys(ctx, "client:*").Result()
    if err != nil {
        log.Fatalf("Error getting keys from Redis: %v", err)
    }

    fmt.Println("Clients in Redis:")

    for _, key := range keys {
        // Получаем данные по ключу
        data, err := rdb.Get(ctx, key).Result()
        if err != nil {
            log.Printf("Error getting value for key %s: %v", key, err)
            continue
        }

        var client objects.Client
        if err := json.Unmarshal([]byte(data), &client); err != nil {
            log.Printf("Error unmarshalling data for key %s: %v", key, err)
            continue
        }

        fmt.Printf("ID: %d, X: %d, Y: %d\n", client.ID, client.X, client.Y)
    }
}

