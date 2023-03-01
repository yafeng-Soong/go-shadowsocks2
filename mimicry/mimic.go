package mimicry

import (
	"container/list"
	"context"
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Filler = iota
	Encapsulation
	Partion
	LastPartion
	PureData
)

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// type Packet struct {
// 	// Id   int
// 	Type int8
// 	Len  int16
// 	Data []byte
// }

// 简单实现，上层应用来的数据都往queue里放，实际发送的包长为max(len(真实包), len(填充包))
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

// 若填充包装不下真实包，直接发送真实包
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

// 终极版，简化了状态并提高了信道利用率
type Encapsulate1 struct {
	net.Conn
	// remain atomic.Int32
	queue            chan []byte
	deque            *list.List
	lock             sync.Mutex
	fillOver         atomic.Bool // 并发安全
	fillerLengthChan chan int
}

func (e *Encapsulate1) Write(b []byte) (int, error) {
	length := len(b)
	tmp := make([]byte, length)
	copy(tmp, b)
	if e.fillOver.Load() {
		e.queue <- tmp
	} else {
		e.saveToDeque(tmp) // 放到队列尾部
	}
	return len(b), nil
}

func (e *Encapsulate1) Read(b []byte) (int, error) {
	return e.Conn.Read(b)
}

func (e *Encapsulate1) ProduceBytes(ctx context.Context) {
	go func() {
		// lens := []int{517, 231, 2957, 1460, 1308, 2236, 38, 2045, 4380, 1460, 1460, 1941, 1460, 1460, 1460, 1460, 1460, 1460, 1460, 1554}
		// for _, x := range lens {
		// 	n := rand.Intn(1000000)
		// 	time.Sleep(time.Duration(n) * time.Microsecond)
		// 	e.fillerLengthChan <- x
		// }
		defer close(e.fillerLengthChan)
		index := rand.Intn(len(FlowList))
		flow := FlowList[index]
		log.Println("使用", flow.Id, flow.Dst, "填充信道")
		for _, x := range flow.Packs {
			if x.Next == -1 {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				e.fillerLengthChan <- x.Length
				time.Sleep(time.Duration(x.Next) * time.Nanosecond)
			}
		}
	}()
}

func (e *Encapsulate1) SendPacks(ctx context.Context) {
	go func() {
		for length := range e.fillerLengthChan {
			select {
			case <-ctx.Done():
				log.Println("填充打断")
				return
			default:
				out := randBytes(length)
				bytes := e.loadFromDeque(length - 3)
				if len(bytes) == 0 {
					out[0] = Filler
				} else {
					out[0] = Encapsulation
					binary.BigEndian.PutUint16(out[1:3], uint16(len(bytes)))
					copy(out[3:], bytes)
				}
				e.Conn.Write(out)
			}
		} // 先用一定数量的随机字节填充信道
		// 清理deque前保证fillOver为true就可以保证清理时没有数据再写入deque
		e.fillOver.Store(true)
		for x := e.deque.Front(); x != nil; {
			select {
			case <-ctx.Done():
				log.Println("清理打断")
				return
			default:
				next := x.Next()
				data := e.deque.Remove(x).([]byte)
				tmp := make([]byte, 1)
				tmp[0] = PureData
				tmp = append(tmp, data...)
				e.Conn.Write(tmp)
				x = next
			}
		}
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

func (e *Encapsulate1) CloseQueue() {
	close(e.queue)
}

// 尽量塞满填充包
func (e *Encapsulate1) loadFromDeque(length int) []byte {
	e.lock.Lock()
	defer e.lock.Unlock()
	res := make([]byte, 0)
	remain := length
	for remain > 0 && e.deque.Len() > 0 {
		tmp := e.deque.Remove(e.deque.Front()).([]byte)
		if len(tmp) <= remain {
			res = append(res, tmp...)
			remain -= len(tmp)
		} else {
			left, right := tmp[:remain], tmp[remain:]
			res = append(res, left...)
			e.deque.PushFront(right)
			remain = 0
		}
	}
	return res
}

func (e *Encapsulate1) saveToDeque(data []byte) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.deque.PushBack(data)
}

func NewEncapsulate1(c net.Conn) *Encapsulate1 {
	en := &Encapsulate1{
		Conn:  c,
		queue: make(chan []byte, 50),
		deque: list.New(),
		// fillOver:         false,
		fillerLengthChan: make(chan int, 50),
	}
	en.fillOver.Store(false)
	return en
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
		tmp := make([]byte, length)
		copy(tmp, b[3:length+3])
		_, err = d.Conn.Write(tmp)
	case PureData:
		// log.Println("得到", n-1, "bytes 原始数据")
		tmp := make([]byte, n-1)
		copy(tmp, b[1:])
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
