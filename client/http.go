package client

import (
	"bufio"
	"errors"
	"fastRPC/conn"
	"fastRPC/html_rpc"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

/*
 * 服务端已经能够接受 CONNECT 请求，并返回了 200 状态码 HTTP/1.0 200 Connected to Gee RPC，
 * 客户端要做的，发起 CONNECT 请求，检查返回状态码即可成功建立连接。
 */

// NewHTTPClient new a Client instance via HTTP as transport protocol
func NewHTTPClient(ncon net.Conn, opt *conn.Option) (*Client, error) {
	_, _ = io.WriteString(ncon, fmt.Sprintf("CONNECT %s HTTP/1.0\n\n", html_rpc.DefaultRPCPath))

	// Require successful HTTP response before switching to RPC protocol.
	resp, err := http.ReadResponse(bufio.NewReader(ncon), &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == html_rpc.Connected {
		return NewClient(ncon, opt)
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	return nil, err
}

// DialHTTP connects to an HTTP RPC server at the specified network address listening on the default HTTP RPC path.
func DialHTTP(network, address string, opts ...*conn.Option) (*Client, error) {
	return dialTimeout(NewHTTPClient, network, address, opts...)
}

// XDial calls different functions to connect to an RPC server according the first parameter rpcAddr.
// rpcAddr is a general format (protocol@addr) to represent a rpc server
// eg, http@10.0.0.1:7001, tcp@10.0.0.1:9999, unix@/tmp/fastrpc.sock
func XDial(rpcAddr string, opts ...*conn.Option) (*Client, error) {
	parts := strings.Split(rpcAddr, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("FastRPC client err: wrong format '%s', expect protocol@addr", rpcAddr)
	}

	protocol, addr := parts[0], parts[1]
	switch protocol {
	case "http":
		return DialHTTP("tcp", addr, opts...)
	default:
		// tcp, unix or other transport protocol
		return Dial(protocol, addr, opts...)
	}
}
