package server

import (
	"encoding/json"
	"fastRPC/conn"
	"io"
	"log"
	"net"
)

// Server represents an RPC Server.
type Server struct{}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer 是一个默认的 Server 实例，主要为了用户使用方便
// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

// Accept accepts connections on the listener and serves requests
// for each incoming connection.
func (server *Server) Accept(lis net.Listener) {
	for {
		// Accept 函数会阻塞程序，直到接收到来自端口的连接
		cliConn, err := lis.Accept()
		if err != nil {
			log.Println("fastRPC server: accept error:", err)
			return
		}

		go server.ServeConn(cliConn)
	}
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection.
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// ServeConn 首先使用 json.NewDecoder 反序列化得到 Option 实例
// 检查 MagicNumber 和 CodeType 的值是否正确
// 然后根据 CodeType 得到对应的消息编解码器，接下来的处理交给 serveRealConn
func (server *Server) ServeConn(cliConn io.ReadWriteCloser) {
	defer func() {
		_ = cliConn.Close()
	}()
	var opt conn.Option
	// 服务端解码报文Option部分
	if err := json.NewDecoder(cliConn).Decode(&opt); err != nil {
		log.Println("fastRPC server: options error: ", err)
	}

	if opt.MagicNumber != conn.MagicNumber {
		log.Printf("fastRPC server: invalid magic number %x", opt.MagicNumber)
		return
	}

	f := conn.NewConnFuncMap[opt.ConnType]
	if f == nil {
		log.Printf("fastRPC server: invalid conn type %s", opt.ConnType)
		return
	}

	// f(conn): 根据用户连接conn，动态生成gob或json类型的连接实例
	server.serveRealConn(f(cliConn))
}
