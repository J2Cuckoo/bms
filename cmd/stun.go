package cmd

import (
	"github.com/pion/stun"
	"log"
	"net"
)

func HandleSTUN(conn net.PacketConn) {
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
