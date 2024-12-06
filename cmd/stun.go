package cmd

import (
	"github.com/pion/stun"
	"log"
	"net"
)

// HandleSTUN 用于处理 STUN 请求
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
		// 确保请求类型是 STUN Binding Request
		if message.Type.Method != stun.MethodBinding {
			log.Printf("Received non-Binding STUN request method: %d from %s", message.Type.Method, srcAddr)
			continue
		}
		// 打印请求的 STUN 消息内容（可选）
		log.Printf("Received STUN Binding Request: %x", message.Raw)
		// 处理 Binding Request 请求
		handleBindingRequest(conn, message, srcAddr)
	}
}

// 处理 Binding Request 并返回 Binding Response
func handleBindingRequest(conn net.PacketConn, message *stun.Message, srcAddr net.Addr) {
	log.Printf("Processing Binding Request from %s", srcAddr)

	// 创建 Binding Response 消息
	response := stun.New()
	response.TransactionID = message.TransactionID
	response.Type = stun.NewType(stun.MethodBinding, stun.ClassSuccessResponse)

	// 1. 添加 MAPPED-ADDRESS 属性
	mappedAddress := &stun.MappedAddress{
		IP:   srcAddr.(*net.UDPAddr).IP,
		Port: srcAddr.(*net.UDPAddr).Port,
	}
	err := mappedAddress.AddTo(response)
	if err != nil {
		log.Printf("Failed to add MAPPED-ADDRESS to Binding Response: %v", err)
		return
	}

	// 2. 添加 XOR-MAPPED-ADDRESS 属性
	xorMappedAddress := &stun.XORMappedAddress{
		IP:   srcAddr.(*net.UDPAddr).IP,
		Port: srcAddr.(*net.UDPAddr).Port,
	}
	err = xorMappedAddress.AddTo(response)
	if err != nil {
		log.Printf("Failed to add XOR-MAPPED-ADDRESS to Binding Response: %v", err)
		return
	}

	// 3. 添加 RESPONSE-ORIGIN 属性
	responseOrigin := &stun.ResponseOrigin{
		IP:   conn.LocalAddr().(*net.UDPAddr).IP,
		Port: conn.LocalAddr().(*net.UDPAddr).Port,
	}
	err = responseOrigin.AddTo(response)
	if err != nil {
		log.Printf("Failed to add RESPONSE-ORIGIN to Binding Response: %v", err)
		return
	}

	// 发送 Binding Response
	// 发送响应
	bytesSent, err := conn.WriteTo(response.Raw, srcAddr)
	if err != nil {
		log.Printf("Failed to send STUN response: %v", err)
		return
	}
	log.Printf("Sent %d bytes in Binding Response to %s", bytesSent, srcAddr)

	// DEBUG: 打印响应的十六进制内容（可选）
	log.Printf("STUN Response: %x", response.Raw)
}

//
//func xorIP(address net.IP, transactionID [12]byte) net.IP {
//	if len(transactionID) < 4 {
//		log.Println("Transaction ID is too short:", len(transactionID))
//		return address // 返回原地址，避免 panic
//	}
//	result := make([]byte, len(address))
//	for i := range address {
//		result[i] = address[i] ^ transactionID[i%4]
//	}
//	return result
//}
//
//// xorPort 对端口号进行 XOR 运算
//func xorPort(port int) int {
//	xorKey := 0x2112 // STUN 协议中规定的 XOR 密钥（2字节）
//	return port ^ xorKey
//}
