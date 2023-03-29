package server

import (
	"fastRPC/html_rpc"
	"io"
	"log"
	"net/http"
)

// ServeHTTP implements a http.Handler that answers RPC requests.
func (server *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodConnect {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = io.WriteString(w, "405 must CONNECT\n")
		return
	}

	// Hijack 可以将一个 http.ResponseWriter 接口转换为一个 net.Conn 接口，这意味着程序可以直接读取和写入底层的 TCP 连接
	nc, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Print("fastRPC hijacking ", req.RemoteAddr, ": ", err.Error())
		return
	}
	_, _ = io.WriteString(nc, "HTTP/1.0 "+html_rpc.Connected+"\n\n")
	server.ServeConn(nc)
}

// HandleHTTP registers an HTTP handler for RPC messages on rpcPath.
// It is still necessary to invoke http.Serve(), typically in a go statement.
func (server *Server) HandleHTTP() {
	http.Handle(html_rpc.DefaultRPCPath, server)

	// for debug
	http.Handle(html_rpc.DefaultDebugPath, debugHTTP{server})
	log.Println("fastRPC server debug path:", html_rpc.DefaultDebugPath)
}

// HandleHTTP is a convenient approach for default server to register HTTP handlers
func HandleHTTP() {
	DefaultServer.HandleHTTP()
}
