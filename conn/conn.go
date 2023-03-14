package conn

import "io"

/*
一个典型的 RPC 调用如下：
err = client.Call("Arith.Multiply", args, &reply)

我们将请求和响应中的参数和返回值抽象为body（args和reply）
剩余的信息放在header（"Arith.Multiply"和err）
*/

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

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

// NewConnFunc 相当于一个函数指针
type NewConnFunc func(closer io.ReadWriteCloser) Conn

var NewConnFuncMap map[Type]NewConnFunc

func init() {
	NewConnFuncMap = make(map[Type]NewConnFunc)

	// 启动时Conn的实例向接口注册
	NewConnFuncMap[GobType] = NewGobConn
	NewConnFuncMap[JsonType] = NewJsonConn
}
