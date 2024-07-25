package cmd

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

// Message 消息结构体
type Message struct {
	Type     string `json:"type"`      // 消息类型: global, room, private
	RoomID   string `json:"room_id"`   // 房间ID
	ClientID string `json:"client_id"` // 客户端ID
	Content  string `json:"content"`   // 消息内容
}

// Room 房间结构体
type Room struct {
	Clients   map[*websocket.Conn]string
	Broadcast chan Message
}

// 全局房间管理
var rooms = make(map[string]*Room)
var roomsLock sync.Mutex

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// 处理WebSocket连接
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	clientID := r.URL.Query().Get("client")
	if roomID == "" || clientID == "" {
		http.Error(w, "Room ID and Client ID are required", http.StatusBadRequest)
		return
	}

	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println("Failed to upgrade connection:", err)
		}
	}(conn)

	// 获取或创建房间
	roomsLock.Lock()
	room, ok := rooms[roomID]
	if !ok {
		room = &Room{
			Clients:   make(map[*websocket.Conn]string),
			Broadcast: make(chan Message),
		}
		rooms[roomID] = room
		go room.run()
	}
	room.Clients[conn] = clientID
	roomsLock.Unlock()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("Unmarshal error:", err)
			continue
		}
		switch msg.Type {
		case "global":
			for _, r := range rooms {
				r.Broadcast <- msg
			}
		case "room":
			room.Broadcast <- msg
		case "private":
			for client, id := range room.Clients {
				if id == msg.ClientID {
					if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
						log.Println("Write error:", err)
						err := client.Close()
						if err != nil {
							return
						}
						delete(room.Clients, client)
					}
				}
			}
		}
	}

	roomsLock.Lock()
	delete(room.Clients, conn)
	if len(room.Clients) == 0 {
		delete(rooms, roomID)
	}
	roomsLock.Unlock()
}

func (r *Room) run() {
	for msg := range r.Broadcast {
		message, err := json.Marshal(msg)
		if err != nil {
			log.Println("Marshal error:", err)
			continue
		}
		for client := range r.Clients {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Println("Write error:", err)
				err := client.Close()
				if err != nil {
					return
				}
				delete(r.Clients, client)
			}
		}
	}
}
