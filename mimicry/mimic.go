package mimicry

import (
	"log"
	"net"
)

type Packet struct {
	Id   int
	Type int
	Data []byte
}

type Encapsulator struct {
	net.Conn
	// remain atomic.Int32
	Buf <-chan *Packet
}

func (e *Encapsulator) Write(b []byte) (int, error) {
	length := len(b)
	tmp := make([]byte, length)
	n := copy(tmp, b)
	log.Println("Write", length, n, "to Encapsulator")
	return e.Conn.Write(tmp)
}

func (e *Encapsulator) Read(b []byte) (int, error) {
	return e.Conn.Read(b)
}

func NewEncapsulator(c net.Conn) net.Conn {
	return &Encapsulator{Conn: c}
}
