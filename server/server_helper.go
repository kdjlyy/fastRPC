package server

import (
	"fastRPC/conn"
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
)

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

/*
serveRealConn 处理用户连接实例
读取请求 readRequest
处理请求 handleRequest
回复请求 sendResponse

在一次连接中，允许接收多个请求，即多个 request header 和 request body，
因此这里使用了 for 无限制地等待请求的到来，直到发生错误（例如连接被关闭，接收到的报文有问题等）

这里需要注意的点有三个：
1. handleRequest 使用了协程并发执行请求；
2. 处理请求是并发的，但是回复请求的报文必须是逐个发送的，并发容易导致多个回复报文交织在一起，客户端无法解析。在这里使用锁(sending)保证；
3. 尽力而为，只有在 header 解析失败时，才终止循环。
*/
func (server *Server) serveRealConn(cc conn.Conn) {
	mutexSendResp := new(sync.Mutex) // make sure to send a complete response
	wg := new(sync.WaitGroup)        // wait until all request are handled

	for {
		req, err := server.readRequest(cc)
		if err != nil {
			// Wait for the request indefinitely until an error occurs,
			// such as the connection is closed or received invalid message, etc.
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, invalidRequest, mutexSendResp)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, mutexSendResp, wg)
	}

	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	header *conn.Header  // header of request
	argv   reflect.Value // argv of request
	replyv reflect.Value // replyv of request
}

func (server *Server) readRequestHeader(cc conn.Conn) (*conn.Header, error) {
	var h conn.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("fastRPC server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc conn.Conn) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}

	req := &request{header: h}

	// TODO: now we don't know the type of request argv, just suppose it's string
	req.argv = reflect.New(reflect.TypeOf(""))

	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("fastRPC server: read argv err:", err)
	}
	return req, nil
}

func (server *Server) sendResponse(cc conn.Conn, h *conn.Header, body interface{}, mutexSendResp *sync.Mutex) {
	mutexSendResp.Lock()
	defer mutexSendResp.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("fastRPC server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc conn.Conn, req *request, mutexSendResp *sync.Mutex, wg *sync.WaitGroup) {
	// TODO, should call registered rpc methods to get the right replyv, now just print argv and send a hello message
	defer wg.Done()

	log.Println("[fastRPC server] request header: ", req.header, ", request argv: ", req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("fastRPC resp %d", req.header.Seq))
	server.sendResponse(cc, req.header, req.replyv.Interface(), mutexSendResp)
}
