package server

import (
	"encoding/json"
	"errors"
	"fastRPC/conn"
	"fastRPC/service"
	"io"
	"log"
	"net"
	"strings"
	"sync"
)

// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer 是一个默认的 Server 实例，主要为了用户使用方便
// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

//func (server *Server) GetServiceMap() *sync.Map {
//	return &server.serviceMap
//}

func (server *Server) Register(this interface{}) error {
	s := service.NewService(this)
	if _, dup := server.serviceMap.LoadOrStore(s.GetName(), s); dup {
		return errors.New("fastRPC: service already defined: " + s.GetName())
	}
	return nil
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(this interface{}) error { return DefaultServer.Register(this) }

// findService 通过 ServiceMethod 从 serviceMap 中找到对应的 service
func (server *Server) findService(serviceMethod string) (svc *service.Service, mType *service.MethodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("fastRPC server: service/method request ill-formed: " + serviceMethod)
		return
	}

	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svcInterface, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("fastRPC server: can't find service: " + serviceName)
		return
	}

	svc = svcInterface.(*service.Service)
	mType = svc.GetMethod(methodName)
	if mType == nil {
		err = errors.New("fastRPC server: can't find method: " + methodName)
	}

	return
}

// ============================================================

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
		log.Println("fastRPC server: option decode error: ", err)
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

	// TODO: 解决粘包问题
	if err := json.NewEncoder(cliConn).Encode(opt); err != nil {
		log.Printf("fastRPC server: option encode error: %s", err.Error())
		return
	}

	// f(conn): 根据用户连接conn，动态生成gob或json类型的连接实例
	server.serveRealConn(f(cliConn), &opt)
}
