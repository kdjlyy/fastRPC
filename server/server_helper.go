package server

import (
	"fastRPC/conn"
	"fastRPC/service"
	"fmt"
	"io"
	"log"
	"reflect"
	"sync"
	"time"
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
func (server *Server) serveRealConn(cc conn.Conn, opt *conn.Option) {
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
		go server.handleRequest(cc, req, mutexSendResp, wg, opt.HandleTimeout)
	}

	wg.Wait()
	_ = cc.Close()
}

// request stores all information of a call
type request struct {
	header *conn.Header  // header of request
	argv   reflect.Value // argv of request
	replyv reflect.Value // replyv of request

	// service
	mType *service.MethodType
	svc   *service.Service
}

func (server *Server) readRequestHeader(cc conn.Conn) (*conn.Header, error) {
	var h conn.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("FastRPC server: read header error:", err)
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
	// search service
	req.svc, req.mType, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}

	req.argv, req.replyv = req.mType.NewArgv(), req.mType.NewReplyv()
	// make sure that argvInterface is a pointer, ReadBody need a pointer as parameter
	argvInterface := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvInterface = req.argv.Addr().Interface()
	}

	if err = cc.ReadBody(argvInterface); err != nil {
		log.Println("FastRPC server: read body err:", err)
	}
	return req, nil
}

func (server *Server) sendResponse(cc conn.Conn, h *conn.Header, body interface{}, mutexSendResp *sync.Mutex) {
	mutexSendResp.Lock()
	defer mutexSendResp.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("FastRPC server: write response error:", err)
	}
}

/*
这里需要确保 sendResponse 仅调用一次，因此将整个过程拆分为 called 和 sent 两个阶段，在这段代码中只会发生如下两种情况：
1. called 管道接收到消息，代表处理没有超时，继续执行 sendResponse。
2. time.After 先于 called 接收到消息，说明处理已经超时，called 和 sent 都将被阻塞。在 case<-time.After(timeout) 处调用 sendResponse
*/
func (server *Server) handleRequest(cc conn.Conn, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	called := make(chan struct{})
	sent := make(chan struct{})
	go func() {
		err := req.svc.Call(req.mType, req.argv, req.replyv)
		called <- struct{}{}
		if err != nil {
			req.header.Error = err.Error()
			server.sendResponse(cc, req.header, invalidRequest, sending)
			sent <- struct{}{}
			return
		}
		server.sendResponse(cc, req.header, req.replyv.Interface(), sending)
		sent <- struct{}{}
	}()

	if timeout == 0 {
		<-called
		<-sent
		return
	}
	select {
	case <-time.After(timeout):
		req.header.Error = fmt.Sprintf("FastRPC server: request handle timeout: expect within %s", timeout)
		server.sendResponse(cc, req.header, invalidRequest, sending)
	case <-called:
		<-sent
	}
}
