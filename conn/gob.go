package conn

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// gob消息的编解码器

type GobConn struct {
	conn    io.ReadWriteCloser // 连接实例，由构造函数传入
	buf     *bufio.Writer      // 防止阻塞而创建的带缓冲的Writer（能提升性能）
	encoder *gob.Encoder       // 编码
	decoder *gob.Decoder       // 解码
}

// ==============================================
// GobConn的方法，GobConn是Conn实例的一种类型
// ==============================================

// ReadHeader 对头部进行解码
func (c *GobConn) ReadHeader(header *Header) error {
	return c.decoder.Decode(header)
}

// ReadBody 对消息体进行解码
func (c *GobConn) ReadBody(body interface{}) error {
	return c.decoder.Decode(body)
}

// Write 写数据
func (c *GobConn) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err := c.encoder.Encode(header); err != nil {
		log.Println("fastRPC conn: gob error while encoding header:", err)
		return err
	}
	if err := c.encoder.Encode(body); err != nil {
		log.Println("fastRPC conn: gob error while encoding body:", err)
		return err
	}

	return nil
}

// Close 关闭连接
func (c *GobConn) Close() error {
	return c.conn.Close()
}

// NewGobConn 构造函数
func NewGobConn(conn io.ReadWriteCloser) Conn {
	buf := bufio.NewWriter(conn)
	return &GobConn{
		conn:    conn,
		buf:     buf,
		decoder: gob.NewDecoder(conn),
		encoder: gob.NewEncoder(buf),
	}
}

// TODO ?
var _ Conn = (*GobConn)(nil)
