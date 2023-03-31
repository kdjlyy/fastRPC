package xclient

import (
	"context"
	"fastRPC/client"
	"fastRPC/conn"
	"io"
	"reflect"
	"sync"
)

// XClient 向用户暴露一个支持负载均衡的客户端
type XClient struct {
	d       Discovery                 // 服务发现实例 Discovery
	mode    SelectMode                // 负载均衡模式 SelectMode
	opt     *conn.Option              // 协议选项 Option
	mu      sync.Mutex                // protect following
	clients map[string]*client.Client // 保存创建成功的 Client 实例
}

var _ io.Closer = (*XClient)(nil)

func NewXClient(d Discovery, mode SelectMode, opt *conn.Option) *XClient {
	return &XClient{d: d, mode: mode, opt: opt, clients: make(map[string]*client.Client)}
}

func (xc *XClient) Close() error {
	xc.mu.Lock()
	defer xc.mu.Unlock()
	for key, c := range xc.clients {
		// I have no idea how to deal with error, just ignore it.
		_ = c.Close()
		delete(xc.clients, key)
	}
	return nil
}

// =========================================================

func (xc *XClient) dial(rpcAddr string) (*client.Client, error) {
	xc.mu.Lock()
	defer xc.mu.Unlock()

	// 检查 xc.clients 是否有缓存的 Client
	// 如果有，检查是否是可用状态，如果是则返回缓存的 Client，如果不可用，则从缓存中删除
	c, ok := xc.clients[rpcAddr]
	if ok && !c.IsAvailable() {
		_ = c.Close()
		delete(xc.clients, rpcAddr)
		c = nil
	}
	// 没有返回缓存的 Client，则说明需要创建新的 Client，缓存并返回
	if c == nil {
		var err error
		c, err = client.XDial(rpcAddr, xc.opt)
		if err != nil {
			return nil, err
		}
		xc.clients[rpcAddr] = c
	}

	return c, nil
}

func (xc *XClient) call(rpcAddr string, ctx context.Context, serviceMethod string, args, reply interface{}) error {
	c, err := xc.dial(rpcAddr)
	if err != nil {
		return err
	}
	return c.Call(ctx, serviceMethod, args, reply)
}

// Call invokes the named function, waits for it to complete, and returns its error status.
// xc will choose a proper server.
func (xc *XClient) Call(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	rpcAddr, err := xc.d.Get(xc.mode)
	if err != nil {
		return err
	}
	return xc.call(rpcAddr, ctx, serviceMethod, args, reply)
}

// Broadcast invokes the named function for every server registered in discovery
// 将请求广播到所有的服务实例，如果任意一个实例发生错误，则返回其中一个错误；如果调用成功，则返回其中一个的结果。有以下几点需要注意：
// 1. 为了提升性能，请求是并发的；
// 2. 并发情况下需要使用互斥锁保证 error 和 reply 能被正确赋值；
// 3. 借助 context.WithCancel 确保有错误发生时，快速失败。
func (xc *XClient) Broadcast(ctx context.Context, serviceMethod string, args, reply interface{}) error {
	servers, err := xc.d.GetAll()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // protect e and replyDone
	var e error
	replyDone := reply == nil // if reply is nil, don't need to set value
	ctx, cancel := context.WithCancel(ctx)

	for _, rpcAddr := range servers {
		wg.Add(1)
		go func(rpcAddr string) {
			defer wg.Done()
			var clonedReply interface{}
			if reply != nil {
				clonedReply = reflect.New(reflect.ValueOf(reply).Elem().Type()).Interface()
			}
			err := xc.call(rpcAddr, ctx, serviceMethod, args, clonedReply)
			mu.Lock()
			if err != nil && e == nil {
				e = err
				cancel() // if any call failed, cancel unfinished calls
			}
			if err == nil && !replyDone {
				// reply = clonedReply
				reflect.ValueOf(reply).Elem().Set(reflect.ValueOf(clonedReply).Elem())
				replyDone = true
			}
			mu.Unlock()
		}(rpcAddr)
	}

	wg.Wait()

	return e
}
