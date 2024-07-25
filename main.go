package main

import (
	"bms/cmd"
	"log"
	"net"
	"net/http"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	wg.Add(3)
	// Start STUN server
	go func() {
		defer wg.Done()
		addr := "0.0.0.0:19302"
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			log.Fatal("Failed to set up STUN server:", err)
		}
		defer func(conn net.PacketConn) {
			err := conn.Close()
			if err != nil {
				log.Fatal("Failed to set up STUN server:", err)
			}
		}(conn)
		log.Println("STUN server started on", addr)
		cmd.HandleSTUN(conn)
	}()

	// Start TURN server
	go func() {
		defer wg.Done()
		// 假设你的 TURN 服务器在 cmd 包中有一个函数 HandleTURN
		err := cmd.HandleTURN()
		if err != nil {
			log.Fatal("Failed to set up TURN server:", err)
		}
		log.Println("TURN server started on")
	}()
	// Start WebSocket server
	go func() {
		defer wg.Done()
		http.HandleFunc("/ws", cmd.HandleWebSocket)
		log.Println("WebSocket server started on :8488")
		err := http.ListenAndServe(":8488", nil)
		if err != nil {
			log.Fatal("ListenAndServe:", err)
		}
	}()

	wg.Wait()
}
