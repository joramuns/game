package objects

import(
    "github.com/gorilla/websocket"
)

type Client struct {
    Conn *websocket.Conn `json:"-"`
    ID   int             `json:"id"`
    X    int             `json:"x"`
    Y    int             `json:"y"`
}

