package client

import (
	"errors"
	"fastRPC/conn"
	"fmt"
	"log"
)

/*
对一个客户端端来说，接收响应、发送请求是最重要的 2 个功能。
首先实现接收功能，接收到的响应有三种情况：
1. call 不存在，可能是请求没有发送完整，或者因为其他原因被取消，但是服务端仍旧处理了。
2. call 存在，但服务端处理出错，即 h.Error 不为空。
3. call 存在，服务端处理正常，那么需要从 body 中读取 Reply 的值。
*/
func (c *Client) receive() {
	var err error
	for err == nil {
		var h conn.Header
		if err = c.cliConn.ReadHeader(&h); err != nil {
			break
		}

		call := c.removeCall(h.Seq)
		switch {
		case call == nil:
			// it usually means that Write partially failed and call was already removed.
			err = c.cliConn.ReadBody(nil)
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = c.cliConn.ReadBody(nil)
			call.done()
		default:
			err = c.cliConn.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}
	// error occurs, so terminateCalls all the pending Call
	c.terminateCalls(err)
}

func newClientConn(cliConn conn.Conn, opt *conn.Option) *Client {
	c := &Client{
		cliConn: cliConn,
		opt:     opt,
		seq:     1, // seq starts with 1, 0 means invalid call
		pending: make(map[uint64]*Call),
	}
	go c.receive()
	return c
}

// ===========================================

// client发送请求
func (c *Client) send(call *Call) {
	// make sure that the client will send a complete request
	c.mutexSendReq.Lock()
	defer c.mutexSendReq.Unlock()

	// register this call.
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = seq
	c.header.Error = ""

	// encode and send the request
	if err := c.cliConn.Write(&c.header, call.Args); err != nil {
		call := c.removeCall(seq)
		// call may be nil, it usually means that Write partially failed,
		// client has received the response and handled
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Go invokes the function asynchronously.
// It returns the Call structure representing the invocation.
// Go 是一个异步接口，返回 call 实例
func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("fastRPC client: done channel is unbuffered")
	}

	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}

	c.send(call)
	return call
}
