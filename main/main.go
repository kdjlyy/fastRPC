package main

import (
	"context"
	"fastRPC/client"
	"fastRPC/server"
	"flag"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var option = flag.String("o", "", "server or client")

type Foo int
type Args struct{ Num1, Num2 int }

// Sum format "func (t *T) MethodName(argType T1, replyType *T2) error"
func (f *Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

func (f *Foo) Mul(args Args, reply *int) error {
	*reply = args.Num1 * args.Num2
	return nil
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime)
	flag.Parse()

	switch *option {
	case "server":
		StartServer()
	case "client":
		StartClient()
	}
}

// StartServer 注册 Foo 到 Server 中，并启动 RPC 服务
func StartServer() {
	var foo Foo
	if err := server.Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}

	// pick a free port
	l, err := net.Listen("tcp", "127.0.0.1:12345")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start FastRPC server on", l.Addr())
	//server.Accept(l)
	server.HandleHTTP()
	_ = http.Serve(l, nil)
}

func StartClient() {
	c, _ := client.DialHTTP("tcp", "127.0.0.1:12345")
	defer func() { _ = c.Close() }()

	// send request & receive response
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx, _ := context.WithTimeout(context.Background(), time.Second)
			args := &Args{Num1: i, Num2: i * i}
			var reply int
			if err := c.Call(ctx, "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}
			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}
