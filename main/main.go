package main

import (
	"encoding/json"
	"fastRPC/conn"
	"fastRPC/server"
	"fmt"
	"log"
	"net"
	"time"
)

/*
1. 在 startServer 中使用了信道 addr，确保服务端端口监听成功，客户端再发起请求。
2. 客户端首先发送 Option 进行协议交换，接下来发送消息头 h := &codec.Header{}，和消息体 geerpc req ${h.Seq}。
3. 最后解析服务端的响应 reply，并打印出来。
*/

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.Accept(l)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	// in fact, following code is like a simple geerpc client
	clientConn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = clientConn.Close() }()

	time.Sleep(time.Second)
	// send options
	_ = json.NewEncoder(clientConn).Encode(server.DefaultOption)
	cc := conn.NewGobConn(clientConn)

	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &conn.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// 对body编码
		_ = cc.Write(h, fmt.Sprintf("test%d", h.Seq))
		// 对头部解码
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("[client] received:", reply)
	}
}
