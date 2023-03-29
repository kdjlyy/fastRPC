package client

import (
	"context"
	"fastRPC/conn"
	"fastRPC/server"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func _assert(condition bool, msg string, v ...interface{}) {
	if !condition {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

// TestClient_dialTimeout 用于测试连接超时。NewClient 函数耗时 2s，ConnectionTimeout 分别设置为 1s 和 0 两种场景
func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")

	f := func(conn net.Conn, opt *conn.Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(time.Second * 2)
		return nil, nil
	}
	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &conn.Option{ConnectTimeout: time.Second})
		_assert(err != nil && strings.Contains(err.Error(), "connect timeout"), "expect a timeout error")
	})
	t.Run("0", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &conn.Option{ConnectTimeout: 0})
		_assert(err == nil, "0 means no limit")
	})
}

// ==========================================

// 用于测试处理超时
// Bar.Timeout 耗时 2s
// 场景一：客户端设置超时时间为 1s，服务端无限制；
// 场景二：服务端设置超时时间为1s，客户端无限制。
type Bar int

func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

func startServer(addr chan string) {
	var b Bar
	_ = server.Register(&b)
	// pick a free port
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	server.Accept(l)
}

func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go startServer(addrCh)
	addr := <-addrCh
	time.Sleep(time.Second)

	t.Run("client timeout", func(t *testing.T) {
		c, _ := Dial("tcp", addr)
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		var reply int
		err := c.Call(ctx, "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "expect a timeout error")
	})

	t.Run("server handle timeout", func(t *testing.T) {
		c, _ := Dial("tcp", addr, &conn.Option{
			HandleTimeout: time.Second,
		})
		var reply int
		err := c.Call(context.Background(), "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), "handle timeout"), "expect a timeout error")
	})
}

// 使用了 unix 协议创建 socket 连接，适用于本机内部的通信，使用上与 TCP 协议并无区别。
func TestXDial(t *testing.T) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		ch := make(chan struct{})
		addr := "/tmp/fastrpc.sock"

		go func() {
			_ = os.Remove(addr)
			l, err := net.Listen("unix", addr)
			if err != nil {
				t.Error("failed to listen unix socket")
				return
			}
			ch <- struct{}{}
			server.Accept(l)
		}()

		<-ch
		_, err := XDial("unix@" + addr)
		_assert(err == nil, "failed to connect unix socket")
	}
}
