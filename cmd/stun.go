package cmd

import (
	"github.com/pion/stun"
	"log"
	"net"
)

func handleSTUN(conn net.PacketConn) {
	buf := make([]byte, 1024)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Println("Read error:", err)
			continue
		}

		message := new(stun.Message)
		err = stun.Decode(buf[:n], message)
		if err != nil {
			log.Println("Failed to decode STUN message:", err)
			continue
		}

		xorAddr := &stun.XORMappedAddress{
			IP:   addr.(*net.UDPAddr).IP,
			Port: addr.(*net.UDPAddr).Port,
		}

		response := stun.MustBuild(stun.TransactionID, stun.BindingSuccess, xorAddr)
		_, err = conn.WriteTo(response.Raw, addr)
		if err != nil {
			log.Println("Write error:", err)
		}
	}
}

func main() {
	addr := "0.0.0.0:3478"
	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		log.Fatal("Failed to set up STUN server:", err)
	}
	defer conn.Close()
	log.Println("STUN server started on", addr)
	handleSTUN(conn)
}
