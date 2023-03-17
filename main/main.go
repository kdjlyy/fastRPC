package main

import (
	"fastRPC/client"
	"fastRPC/server"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

func startServer() {
	// pick a free port
	l, err := net.Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start fastRPC server on", l.Addr())
	server.Accept(l)
}

func main() {
	log.SetFlags(0)
	go startServer()

	// 等待server拉起
	time.Sleep(time.Second * 2)

	c, _ := client.Dial("tcp", "127.0.0.1:12345")
	defer func() { _ = c.Close() }()

	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("fastRPC req %d", i)
			var reply string
			if err := c.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
}
