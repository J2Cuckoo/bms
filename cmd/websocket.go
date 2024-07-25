package cmd

import (
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
)

// Room 房间结构体
type Room struct {
	Clients   map[*websocket.Conn]bool
	Broadcast chan []byte
}

// 全局房间管理
var rooms = make(map[string]*Room)
var roomsLock sync.Mutex

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// HandleWebSocket 处理WebSocket连接
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 获取房间ID
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
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
			log.Fatal("websocket conn fail ", err)
		}
	}(conn)
	// 获取或创建房间
	roomsLock.Lock()
	room, ok := rooms[roomID]
	if !ok {
		room = &Room{
			Clients:   make(map[*websocket.Conn]bool),
			Broadcast: make(chan []byte),
		}
		rooms[roomID] = room
		go room.run()
	}
	room.Clients[conn] = true
	roomsLock.Unlock()

	// 处理消息
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		if string(message) == "global" {
			// 全局广播
			for _, r := range rooms {
				r.Broadcast <- message
			}
		} else {
			// 房间广播
			room.Broadcast <- message
		}
	}
	// 移除连接
	roomsLock.Lock()
	delete(room.Clients, conn)
	if len(room.Clients) == 0 {
		delete(rooms, roomID)
	}
	roomsLock.Unlock()
}

func (r *Room) run() {
	for {
		message := <-r.Broadcast
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
