package cmd

import (
	"github.com/pion/stun"
	"log"
	"net"
)

func HandleSTUN(conn net.PacketConn) {
	for {
		// 读取 UDP 数据包
		buf := make([]byte, 1500)
		n, srcAddr, err := conn.ReadFrom(buf)
		if err != nil {
			log.Printf("Failed to read from connection: %v", err)
			continue
		}
		// 创建 STUN 消息实例，并将读取的数据赋值给 message.Raw
		message := new(stun.Message)
		message.Raw = buf[:n]

		// 尝试解码 STUN 消息
		err = message.Decode()
		if err != nil {
			log.Printf("Failed to decode STUN message from %s: %v", srcAddr, err)
			continue
		}

		// 检查是否为 Binding Request
		if message.Type.Method == stun.MethodBinding {
			log.Printf("Received Binding Request from %s", srcAddr)

			// 创建 Binding Response 消息
			response := stun.New()
			response.TransactionID = message.TransactionID
			response.Type = stun.NewType(stun.MethodBinding, stun.ClassSuccessResponse)

			// 添加 MAPPED-ADDRESS 属性
			mappedAddress := &stun.XORMappedAddress{
				IP:   srcAddr.(*net.UDPAddr).IP,
				Port: srcAddr.(*net.UDPAddr).Port,
			}
			err = mappedAddress.AddTo(response)
			if err != nil {
				log.Printf("Failed to add MAPPED-ADDRESS to response: %v", err)
				continue
			}

			// 发送响应
			bytesSent, err := conn.WriteTo(response.Raw, srcAddr)
			if err != nil {
				log.Printf("Failed to send STUN response: %v", err)
				continue
			}

			// 确认数据已发送
			log.Printf("Sent %d bytes in Binding Response to %s", bytesSent, srcAddr)

			// DEBUG: 打印响应的十六进制内容（可选）
			log.Printf("STUN Response: %x", response.Raw)
		} else {
			log.Printf("Received non-Binding Request from %s", srcAddr)
		}
	}
}
