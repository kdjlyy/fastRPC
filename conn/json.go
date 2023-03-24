package conn

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// json消息的编解码器

type JsonConn struct {
	conn    io.ReadWriteCloser // 连接实例，由构造函数传入
	buf     *bufio.Writer      // 防止阻塞而创建的带缓冲的Writer（能提升性能）
	encoder *gob.Encoder       // 编码
	decoder *gob.Decoder       // 解码
}

// ReadHeader 对头部进行解码
func (c *JsonConn) ReadHeader(header *Header) error {
	return c.decoder.Decode(header)
}

// ReadBody 对消息体进行解码
func (c *JsonConn) ReadBody(body interface{}) error {
	return c.decoder.Decode(body)
}

// Write 写数据
func (c *JsonConn) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.encoder.Encode(header); err != nil {
		log.Println("fastRPC conn: json error while encoding header:", err)
		return err
	}
	if err := c.encoder.Encode(body); err != nil {
		log.Println("fastRPC conn: json error while encoding body:", err)
		return err
	}

	return nil
}

// Close 关闭连接
func (c *JsonConn) Close() error {
	return c.conn.Close()
}

// NewJsonConn 构造函数
func NewJsonConn(conn io.ReadWriteCloser) Conn {
	buf := bufio.NewWriter(conn)
	return &JsonConn{
		conn:    conn,
		buf:     buf,
		decoder: gob.NewDecoder(conn),
		encoder: gob.NewEncoder(buf),
	}
}

// // 将nil转换为*JsonConn类型，然后再转换为Conn接口，如果转换失败，说明*JsonConn没有实现Conn接口的所有方法。
var _ Conn = (*JsonConn)(nil)
