package client

import "io"

// IsAvailable return true while the client available currently
func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

// NotAvailable return true while the client not available currently
func (c *Client) NotAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.shutdown || c.closing
}

// Close client the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing {
		return ErrConnClosed
	}
	c.closing = true
	return c.cliConn.Close()
}

var _ io.Closer = (*Client)(nil)

// ============================================================

// registerCall 注册RPC调用方法
// 将参数 call 添加到 client.pending 中，并更新 client.seq
func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.shutdown || c.closing {
		return 0, ErrConnNotAvailable
	}

	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

// removeCall 关闭RPC调用方法
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()

	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// terminateCalls
// 服务端或客户端发生错误时调用，将 shutdown 设置为 true，且将错误信息通知所有 pending 状态的 call
func (c *Client) terminateCalls(err error) {
	c.mutexSendReq.Lock()
	defer c.mutexSendReq.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}
