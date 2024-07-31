package objects

import(
    "github.com/gorilla/websocket"
)

type Client struct {
    Conn *websocket.Conn `json:"-"`
    Player  *Object
    ID      int             `json:"id"`
}

