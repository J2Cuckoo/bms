package main

import (
	"log"
	"os/exec"
	"sync"
)

func startServer(name string, cmdStr string) {
	cmd := exec.Command("go", "run", cmdStr)
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start %s: %v", name, err)
	}
	log.Printf("%s server started", name)
	err = cmd.Wait()
	if err != nil {
		log.Fatalf("%s server stopped: %v", name, err)
	}
}

func main() {
	var wg sync.WaitGroup
	servers := map[string]string{
		"WebSocket": "E:/go/rts/cmd/webSocket/main.go",
		"STUN":      "E:/go/rts/cmd/stun/main.go",
		"TURN":      "E:/go/rts/cmd/turn/main.go",
	}

	for name, cmdStr := range servers {
		wg.Add(1)
		go func(name, cmdStr string) {
			defer wg.Done()
			startServer(name, cmdStr)
		}(name, cmdStr)
	}

	wg.Wait()
}
