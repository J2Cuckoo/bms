package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/tjfoc/gmsm/sm3"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// Message 消息结构体
type Message struct {
	Type     string      `json:"type,omitempty"`     // 消息类型: global, room, private, join, init
	RoomID   string      `json:"roomId,omitempty"`   // 房间ID
	ClientID string      `json:"clientId,omitempty"` // 客户端ID
	Content  interface{} `json:"content,omitempty"`  // 消息内容，支持任意类型
}

// Room 房间结构体
type Room struct {
	Clients   map[*websocket.Conn]string
	Broadcast chan Message
}

// 全局房间管理
var rooms = make(map[string]*Room)
var roomsLock sync.Mutex

// 全局的客户端ID列表
var clientIDs []string
var clientIDsLock sync.Mutex

var upgrade = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源
	},
}

// HandleWebSocket 处理WebSocket连接
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrade.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade connection:", err)
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println("Failed to close connection:", err)
		}
	}(conn)

	// 启动心跳机制
	go pingClient(conn)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			handleReadError(err)
			break
		}
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Println("Unmarshal error:", err)
			continue
		}
		roomsLock.Lock()
		clientIDsLock.Lock()
		switch msg.Type {
		case "init":
			clientID := generateClientID(msg.Content.(string))
			clientIDs = append(clientIDs, clientID)
			// 检查是否存在 "global" 房间，如果不存在则创建
			globalRoom, ok := rooms["global"]
			if !ok {
				globalRoom = &Room{
					Clients:   make(map[*websocket.Conn]string),
					Broadcast: make(chan Message),
				}
				rooms["global"] = globalRoom
				go globalRoom.run()
			}
			// 将客户端加入 "global" 房间
			globalRoom.Clients[conn] = clientID
			// 生成响应消息
			response := Message{Type: "init", ClientID: clientID}
			responseBytes, _ := json.Marshal(response)
			err = conn.WriteMessage(websocket.TextMessage, responseBytes)
			if err != nil {
				log.Println("Write error:", err)
				roomsLock.Unlock()
				clientIDsLock.Unlock()
				return
			}
		case "join":
			if msg.RoomID == "" {
				log.Println("Join error: RoomID is required")
				roomsLock.Unlock()
				clientIDsLock.Unlock()
				continue
			}
			room, ok := rooms[msg.RoomID]
			if !ok {
				room = &Room{
					Clients:   make(map[*websocket.Conn]string),
					Broadcast: make(chan Message),
				}
				rooms[msg.RoomID] = room
				go room.run()
			}
			room.Clients[conn] = msg.ClientID
			// 获取当前房间中所有客户端的ID
			response := map[string]interface{}{
				"type":      "join",
				"content":   "join room successful",
				"clientIds": clientIDs,
			}
			filteredResponse := filterEmptyFields(response)
			responseBytes, _ := json.Marshal(filteredResponse)
			err = conn.WriteMessage(websocket.TextMessage, responseBytes)
			if err != nil {
				log.Println("Write error:", err)
				roomsLock.Unlock()
				clientIDsLock.Unlock()
				return
			}
		case "global":
			// 获取 "global" 房间
			globalRoom, ok := rooms["global"]
			// 获取发送者的 clientID
			senderClientID := ""
			for client, id := range globalRoom.Clients {
				if client == conn {
					senderClientID = id
					break
				}
			}

			if senderClientID == "" {
				log.Println("Sender clientID not found")
				roomsLock.Unlock()
				clientIDsLock.Unlock()
				return
			}

			if ok {
				log.Println("Processing global room messages")
				for client := range globalRoom.Clients {
					log.Println("Connection in the global room:", client)
					// 只发送给除发送者外的其他连接
					if globalRoom.Clients[client] != senderClientID {
						err := client.WriteMessage(websocket.TextMessage, message)
						if err != nil {
							log.Println("Write error:", err)
							err := client.Close()
							if err != nil {
								roomsLock.Unlock()
								clientIDsLock.Unlock()
								return
							}
							delete(globalRoom.Clients, client)
						}
					}
				}
			} else {
				log.Println("Global room not found")
			}
		case "room":
			room, ok := rooms[msg.RoomID]
			if ok {
				for client, id := range room.Clients {
					senderClientID := ""
					if client == conn {
						senderClientID = id
						break
					}
					if room.Clients[client] != senderClientID {
						err := client.WriteMessage(websocket.TextMessage, message)
						if err != nil {
							log.Println("Write error:", err)
							err := client.Close()
							if err != nil {
								roomsLock.Unlock()
								clientIDsLock.Unlock()
								return
							}
							delete(room.Clients, client)
						}
					}
				}
			}

		case "private":
			for _, room := range rooms {
				for client, id := range room.Clients {
					if id == msg.ClientID {
						if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
							log.Println("Write error:", err)
							err := client.Close()
							if err != nil {
								roomsLock.Unlock()
								clientIDsLock.Unlock()
								return
							}
							delete(room.Clients, client)
						}
					}
				}
			}
		}
		clientIDsLock.Unlock()
		roomsLock.Unlock()
	}

	roomsLock.Lock()
	clientIDsLock.Unlock()
	for roomID, room := range rooms {
		delete(room.Clients, conn)
		if len(room.Clients) == 0 {
			delete(rooms, roomID)
		}
	}
	clientIDsLock.Unlock()
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

// generateClientID 生成一个固定的7位数作为clientID
func generateClientID(macAddress string) string {
	counter := 0
	for {
		hash := sm3.New()
		// 将macAddress和counter组合在一起
		hash.Write([]byte(fmt.Sprintf("%s%d", macAddress, counter)))
		hashBytes := hash.Sum(nil)
		hashString := hex.EncodeToString(hashBytes)
		// 将hashString的前几位转换为数字
		num := new(big.Int)
		num.SetString(hashString[:16], 16) // 选择16个字符作为样本，可以根据需要调整

		clientID := fmt.Sprintf("%09d", num.Uint64()%1000000000)
		if clientID[0] != '0' {
			return clientID
		}
		counter++
	}
}

// pingClient 向客户端发送心跳包以保持连接活跃
func pingClient(conn *websocket.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			log.Println("Ping error:", err)
			err := conn.Close()
			if err != nil {
				return
			}
			break
		}
	}
}

// handleReadError 处理读消息时的错误
func handleReadError(err error) {
	if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
		log.Println("Client disconnected:", err)
	} else {
		log.Println("Read error:", err)
	}
}

// filterEmptyFields 过滤掉值为空字符串的字段
func filterEmptyFields(data map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for key, value := range data {
		switch v := value.(type) {
		case string:
			if v != "" {
				filtered[key] = value
			}
		case []interface{}:
			if len(v) > 0 {
				filtered[key] = value
			}
		default:
			filtered[key] = value
		}
	}
	return filtered
}
