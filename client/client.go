package client

import (
	"context"
	"encoding/json"
	"errors"
	"fastRPC/conn"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

var (
	ErrConnClosed       = errors.New("connection already closed")
	ErrConnNotAvailable = errors.New("connection not available")
)

// Client 客户端最核心部分
// Client represents an RPC Client.
// There may be multiple outstanding Calls associated with a single Client,
// and a Client may be used by multiple goroutines simultaneously.
type Client struct {
	cliConn conn.Conn    // 客户端的RPC连接
	opt     *conn.Option // 消息编码方式

	// 为了保证请求的有序发送的互斥锁，即防止出现多个请求报文混淆
	mutexSendReq sync.Mutex

	// header 是每个请求的消息头，header 只有在请求发送时才需要，而请求发送是互斥的
	// 因此每个客户端只需要一个，声明在 Client 结构体中可以复用。
	header conn.Header

	mu       sync.Mutex       // Client资源的互斥锁
	seq      uint64           // 用于给发送的请求编号，每个请求拥有唯一编号 seq starts with 1, 0 means invalid call
	pending  map[uint64]*Call // 存储未处理完的请求，键是编号，值是 Call 实例
	closing  bool             // user has called Close()
	shutdown bool             // server has told us to stop
}

// NewClient Client构造函数
// 创建 Client 实例时，首先需要完成一开始的协议交换，即发送 Option 信息给服务端。
// 协商好消息的编解码方式之后，再创建一个子协程调用 receive() 接收响应。
func NewClient(nc net.Conn, opt *conn.Option) (*Client, error) {
	f := conn.NewConnFuncMap[opt.ConnType]
	if f == nil {
		err := fmt.Errorf("invalid connection type %s", opt.ConnType)
		log.Println("FastRPC client: connection type error:", err)
		return nil, err
	}

	// send options with server
	if err := json.NewEncoder(nc).Encode(opt); err != nil {
		log.Println("FastRPC client: option encode error: ", err)
		_ = nc.Close()
		return nil, err
	}

	// TODO: 解决粘包问题
	if err := json.NewDecoder(nc).Decode(opt); err != nil {
		log.Printf("FastRPC client: option decode error: %s", err.Error())
		_ = nc.Close()
		return nil, err
	}

	return newClientConn(f(nc), opt), nil
}

type newClientFunc func(nc net.Conn, opt *conn.Option) (client *Client, err error)

// dialTimeout 超时处理 将 NewClient 作为入参，在2个地方添加了超时处理的机制：
// 1. net.DialTimeout，如果连接创建超时，将返回错误
// 2. 使用子协程执行 NewClient，执行完成后则通过管道ch发送结果，如果 time.After() 管道先接收到消息，则说明 NewClient 执行超时，返回错误
func dialTimeout(f newClientFunc, network, address string, opts ...*conn.Option) (client *Client, err error) {
	opt, err := conn.ParseOptions(opts...)
	if err != nil {
		return nil, err
	}
	nc, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}

	// close the connection if client is nil
	defer func() {
		if err != nil {
			_ = nc.Close()
		}
	}()

	type clientResult struct {
		client *Client
		err    error
	}
	ch := make(chan clientResult)
	go func() {
		client, err := f(nc, opt)
		ch <- clientResult{client: client, err: err}
	}()

	if opt.ConnectTimeout == 0 {
		result := <-ch
		return result.client, result.err
	}
	select {
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("FastRPC client: connect timeout: expect within %s", opt.ConnectTimeout)
	case result := <-ch:
		return result.client, result.err
	}
}

// Dial connects to an RPC server at the specified network address
// 实现 Dial 函数，便于用户传入服务端地址，创建 Client 实例。
func Dial(network, address string, opts ...*conn.Option) (*Client, error) {
	return dialTimeout(NewClient, network, address, opts...)
}

// Call invokes the named function, waits for it to complete, and returns its error status.
/*
   Call 是对 Go 的封装，阻塞读取管道 call.Done，等待响应返回，是一个同步接口
   Call 的超时处理机制，使用 context 包实现，控制权交给用户，控制更为灵活
   用户可以使用 context.WithTimeout 创建具备超时检测能力的 context 对象来控制。例如：

	ctx, _ := context.WithTimeout(context.Background(), time.Second)
	var reply int
 	err := client.Call(ctx, "Foo.Sum", &Args{1, 2}, &reply)
*/
func (c *Client) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	call := c.Go(serviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		c.removeCall(call.Seq)
		return errors.New("FastRPC client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}
}

//func (c *Client) Call(serviceMethod string, args, reply interface{}) error {
//	call := <-c.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
//	return call.Error
//}
