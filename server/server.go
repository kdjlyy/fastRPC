package server

import (
	"encoding/json"
	"fastRPC/conn"
	"io"
	"log"
	"net"
)

/*
客户端与服务端的通信需要协商一些内容
目前fastRPC需要协商的唯一一项内容是消息的编解码方式。我们将这部分信息，放到结构体 Option 中承载

服务端收到的报文格式：
| Option{MagicNumber: xxx, ConnType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
服务端首先使用 JSON 解码 Option，然后通过 Option 的 CodeType 解码剩余的内容

在一次连接中，Option 固定在报文的最开始，Header 和 Body 可以有多个，即报文可能是这样的
| Option | Header1 | Body1 | Header2 | Body2 | ...
*/

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber int       // MagicNumber 标记这是一个fastRPC请求
	ConnType    conn.Type // ConnType 支持GobType和JsonType
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	ConnType:    conn.GobType,
}

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
	var opt Option
	// 服务端解码报文Option部分
	if err := json.NewDecoder(cliConn).Decode(&opt); err != nil {
		log.Println("fastRPC server: options error: ", err)
	}

	if opt.MagicNumber != MagicNumber {
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
