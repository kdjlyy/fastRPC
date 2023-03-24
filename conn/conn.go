package conn

import (
	"io"
	"time"
)

/*
一个典型的 RPC 调用如下：
err = client.Call("Arith.Multiply", args, &reply)
对 net/rpc 而言，一个函数需要能够被远程调用，需要满足如下条件：
func (t *T) MethodName(argType T1, replyType *T2) error

我们将请求和响应中的参数和返回值抽象为body（args和reply）
剩余的信息放在header（"Arith.Multiply"和err）

服务端收到的报文格式：
| Option{MagicNumber: xxx, ConnType: xxx} | Header{ServiceMethod ...} | Body interface{} |
| <------      固定 JSON 编码      ------>  | <-------   编码方式由 CodeType 决定   ------->|
服务端首先使用 JSON 解码 Option，然后通过 Option 的 CodeType 解码剩余的内容

在一次连接中，Option 固定在报文的最开始，Header 和 Body 可以有多个，即报文可能是这样的
| Option | Header1 | Body1 | Header2 | Body2 | ...
*/

type Type string

const (
	MagicNumber      = 0x3bef5c
	GobType     Type = "application/gob"
	JsonType    Type = "application/json"
)

// Option 客户端与服务端的通信需要协商一些内容
// 目前fastRPC需要协商的唯一一项内容是消息的编解码方式。我们将这部分信息，放到结构体 Option 中承载
type Option struct {
	MagicNumber int  // MagicNumber 标记这是一个fastRPC请求
	ConnType    Type // ConnType 支持GobType和JsonType

	// for timeout operation
	ConnectTimeout time.Duration // 0 means no limit
	HandleTimeout  time.Duration
}

var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	ConnType:       GobType,
	ConnectTimeout: time.Second * 10,
}

type Header struct {
	// name of service or method, e.g. "Service.Method"
	ServiceMethod string
	// request's sequence number, used to distinguish different requests
	Seq uint64
	// if an error occurs on server side, the error message will be put in Error
	// on client side, Error should be null in the beginning
	Error string
}

// Conn 抽象出对消息体进行编解码的接口 Conn，抽象出接口是为了实现不同的 Conn 实例
type Conn interface {
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
	io.Closer
}

// NewConnFunc 相当于一个函数指针
type NewConnFunc func(closer io.ReadWriteCloser) Conn

var NewConnFuncMap map[Type]NewConnFunc

func init() {
	NewConnFuncMap = make(map[Type]NewConnFunc)

	// 启动时Conn的实例向接口注册
	NewConnFuncMap[GobType] = NewGobConn
	NewConnFuncMap[JsonType] = NewJsonConn
}
