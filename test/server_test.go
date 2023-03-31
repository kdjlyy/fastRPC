package test

import (
	"encoding/json"
	"fastRPC/conn"
	"fastRPC/server"
	"fmt"
	"log"
	"net"
	"testing"
	"time"
)

/*
1. 在 startServer 中使用了信道 addr，确保服务端端口监听成功，客户端再发起请求。
2. 客户端首先发送 Option 进行协议交换，接下来发送消息头 h := &conn.Header{}，和消息体 fastRPC req ${h.Seq}。
3. 最后解析服务端的响应 reply，并打印出来。
*/

func startServer() {
	// pick a free port
	l, err := net.Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start FastRPC server on", l.Addr())
	server.Accept(l)
}

func TestFastRpcServer(t *testing.T) {
	go startServer()

	// =======================================
	// ====== a simple FastRPC client ========
	// =======================================
	time.Sleep(time.Second * 2) // wait for server established
	clientConn, _ := net.Dial("tcp", "127.0.0.1:12345")
	defer func() { _ = clientConn.Close() }()

	// 设置options
	_ = json.NewEncoder(clientConn).Encode(conn.DefaultOption)
	cc := conn.NewGobConn(clientConn)

	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &conn.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}

		//
		// send request
		//
		_ = cc.Write(h, fmt.Sprintf("test%d", h.Seq))
		time.Sleep(time.Second)

		//
		// receive response
		//
		var replyHeader conn.Header
		_ = cc.ReadHeader(&replyHeader) // 解码Header
		var replyBody string
		_ = cc.ReadBody(&replyBody) // 解码Body
		log.Println("[client] received:", replyHeader, replyBody)
	}
}
