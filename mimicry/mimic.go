package mimicry

import (
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"time"
)

const (
	Filler = iota
	Encapsulation
	PureData
)

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// type Packet struct {
// 	// Id   int
// 	Type int8
// 	Len  int16
// 	Data []byte
// }

type Encapsulator struct {
	net.Conn
	// remain atomic.Int32
	queue            chan []byte
	FillerLengthChan chan int
}

func (e *Encapsulator) Write(b []byte) (int, error) {
	length := len(b)
	tmp := make([]byte, length)
	copy(tmp, b)
	e.queue <- tmp
	// log.Println("拷贝", length, "bytes 数据到queue中")
	return length, nil
}

func (e *Encapsulator) Read(b []byte) (int, error) {
	return e.Conn.Read(b)
}

func (e *Encapsulator) ProduceBytes() {
	go func() {
		lens := []int{517, 231, 2957, 1460, 1308, 2236, 38, 2045, 4380, 1460, 1460, 1941, 1460, 1460, 1460, 1460, 1460, 1460, 1460, 1554}
		for _, x := range lens {
			n := rand.Intn(1000000)
			time.Sleep(time.Duration(n) * time.Microsecond)
			e.FillerLengthChan <- x
		}
		close(e.FillerLengthChan)
	}()
}

func (e *Encapsulator) SendPacks() {
	go func() {
		for length := range e.FillerLengthChan {
			// out := make([]byte, length)
			var out []byte
			select {
			case data := <-e.queue:
				out = packageBuf(data, length)
			default:
				out = randBytes(length)
				out[0] = Filler
			}
			e.Conn.Write(out)
		} // 先用一定数量的随机字节填充信道
		log.Println("填充完毕")
		// 随机字节发送完后填充真实数据
		for data := range e.queue {
			tmp := make([]byte, 1)
			tmp[0] = PureData
			tmp = append(tmp, data...)
			e.Conn.Write(tmp)
		}
	}()
}

func (e *Encapsulator) CloseQueue() {
	close(e.queue)
}

// func packageBuf(src []byte, length int) (out []byte) {
// 	if len(src)+3 > length {
// 		length = len(src) + 3
// 	}
// 	out = randBytes(length)
// 	out[0] = Encapsulation
// 	binary.BigEndian.PutUint16(out[1:3], uint16(len(src)))
// 	copy(out[3:], src)
// 	return
// }

func packageBuf(src []byte, length int) (out []byte) {
	if len(src)+3 > length {
		out = make([]byte, 1)
		out[0] = PureData
		out = append(out, src...)
	} else {
		out = randBytes(length)
		out[0] = Encapsulation
		binary.BigEndian.PutUint16(out[1:3], uint16(len(src)))
		copy(out[3:], src)
	}
	return
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return b
}

func NewEncapsulator(c net.Conn) *Encapsulator {
	return &Encapsulator{
		Conn:             c,
		queue:            make(chan []byte, 50),
		FillerLengthChan: make(chan int, 50),
	} // 最大50个包的缓冲
}

type Decapsulator struct {
	net.Conn
}

func (d *Decapsulator) Read(b []byte) (int, error) {
	return d.Conn.Read(b)
}

func (d *Decapsulator) Write(b []byte) (int, error) {
	n := len(b)
	var err error = nil
	switch b[0] {
	case Filler:
		// 填充数据直接扔掉
		// log.Println("扔掉 ", n, "bytes 填充数据")
	case Encapsulation:
		length := binary.BigEndian.Uint16(b[1:3])
		// log.Println("得到", length, "bytes 解封数据")
		// tmp := make([]byte, length)
		// copy(tmp, b[3:length+3])
		_, err = d.Conn.Write(b[3 : length+3])
	case PureData:
		// log.Println("得到", n-1, "bytes 原始数据")
		_, err = d.Conn.Write(b[1:])
	}
	if err != nil {
		log.Println("type", b[0], ", write", n, "bytes fall:", err.Error())
	}
	return n, err
}

func NewDecapsulator(c net.Conn) net.Conn {
	return &Decapsulator{Conn: c}
}
