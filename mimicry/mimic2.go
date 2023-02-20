package mimicry

import (
	"container/list"
	"encoding/binary"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

// 简单实现的改进版，填充阶段的真实数据包放到双向链表中，填充完毕后放到queue中；状态比较复杂，遂舍弃
type Encapsulate struct {
	id string
	net.Conn
	// remain atomic.Int32
	queue            chan []byte
	deque            *list.List
	lock             sync.Mutex
	fillOver         bool
	isPartion        bool
	fillerLengthChan chan int
}

func (e *Encapsulate) Write(b []byte) (int, error) {
	length := len(b)
	tmp := make([]byte, length)
	copy(tmp, b)
	if e.fillOver {
		e.queue <- tmp
	} else {
		e.saveToDeque(tmp) // 放到队列尾部
	}
	// log.Println(e.id, "放入", length, "bytes数据", hex.EncodeToString(tmp))
	return length, nil
}

func (e *Encapsulate) Read(b []byte) (int, error) {
	return e.Conn.Read(b)
}

func (e *Encapsulate) ProduceBytes() {
	go func() {
		lens := []int{517, 231, 2957, 1460, 1308, 2236, 38, 2045, 4380, 1460, 1460, 1941, 1460, 1460, 1460, 1460, 1460, 1460, 1460, 1554}
		for _, x := range lens {
			n := rand.Intn(1000000)
			time.Sleep(time.Duration(n) * time.Microsecond)
			e.fillerLengthChan <- x
		}
		close(e.fillerLengthChan)
		e.fillOver = true
	}()
}

func (e *Encapsulate) SendPacks() {
	go func() {
		for length := range e.fillerLengthChan {
			out := e.loadFromDeque(length)
			e.Conn.Write(out)
		} // 先用一定数量的随机字节填充信道
		// 清理deque
		e.lock.Lock()
		log.Println("清理数据")
		for x := e.deque.Front(); x != nil; x = x.Next() {
			data := x.Value.([]byte)
			if e.isPartion {
				tmp := make([]byte, 3)
				tmp[0] = LastPartion
				binary.BigEndian.PutUint16(tmp[1:3], uint16(len(data)))
				tmp = append(tmp, data...)
				e.Conn.Write(tmp)
				e.isPartion = false
				continue
			}
			tmp := make([]byte, 1)
			tmp[0] = PureData
			tmp = append(tmp, data...)
			e.Conn.Write(tmp)
		}
		e.lock.Unlock()
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

func (e *Encapsulate) CloseQueue() {
	close(e.queue)
}

// 若填充包装不下真实包，则装一部分，并把剩下部分放回双向链表头
func (e *Encapsulate) loadFromDeque(length int) []byte {
	e.lock.Lock()
	defer e.lock.Unlock()
	var res []byte
	if e.deque.Len() == 0 {
		res = randBytes(length)
		res[0] = Filler
		return res
	}
	tmp := e.deque.Remove(e.deque.Front()).([]byte)
	if len(tmp)+3 <= length {
		res = randBytes(length)
		res[0] = Encapsulation
		binary.BigEndian.PutUint16(res[1:3], uint16(len(tmp)))
		copy(res[3:], tmp)
		if e.isPartion {
			res[0] = LastPartion
			e.isPartion = false
		}
	} else {
		res = make([]byte, length)
		res[0] = Partion
		binary.BigEndian.PutUint16(res[1:3], uint16(length-3))
		copy(res[3:], tmp)
		e.deque.PushFront(tmp[length-3:])
		e.isPartion = true
	}
	// log.Println(e.id, "发送", res[0], "|", binary.BigEndian.Uint16(res[1:3]), "|", len(res))
	// log.Println("期望读取", length, "bytes，实际读取", len(res), "bytes")
	return res
}

func (e *Encapsulate) saveToDeque(data []byte) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.deque.PushBack(data)
}

func NewEncapsulate(c net.Conn) *Encapsulate {
	return &Encapsulate{
		id:               string(randBytes(7)),
		Conn:             c,
		queue:            make(chan []byte, 50),
		deque:            list.New(),
		fillOver:         false,
		isPartion:        false,
		fillerLengthChan: make(chan int, 50),
	}
}

type Decapsulate struct {
	id string
	net.Conn
	partionBuf []byte
}

func (d *Decapsulate) Read(b []byte) (int, error) {
	return d.Conn.Read(b)
}

func (d *Decapsulate) Write(b []byte) (int, error) {
	n := len(b)
	var err error = nil
	switch b[0] {
	case Filler:
		// 填充数据直接扔掉
	case Encapsulation:
		length := binary.BigEndian.Uint16(b[1:3])
		// log.Println(d.id, "得到", length, "bytes 解封数据", hex.EncodeToString(b[3:length+3]))
		// tmp := make([]byte, length)
		// copy(tmp, b[3:length+3])
		_, err = d.Conn.Write(b[3 : length+3])
	case Partion:
		d.partionBuf = append(d.partionBuf, b[3:]...)
		// length := binary.BigEndian.Uint16(b[1:3])
		// log.Println(d.id, "得到", length, "bytes 分区数据，已组装数据长度", len(d.partionBuf))
	case LastPartion:
		length := binary.BigEndian.Uint16(b[1:3])
		d.partionBuf = append(d.partionBuf, b[3:length+3]...)
		// log.Println(d.id, "得到", length, "bytes 分区数据，组装好分区长度", len(d.partionBuf), hex.EncodeToString(d.partionBuf))
		_, err = d.Conn.Write(d.partionBuf)
		d.partionBuf = make([]byte, 0)
	case PureData:
		// log.Println(d.id, "得到", n-1, "bytes 原始数据", hex.EncodeToString(b[1:]))
		_, err = d.Conn.Write(b[1:])
	}
	if err != nil {
		log.Println("type", b[0], ", write", n, "bytes fall:", err.Error())
	}
	return n, err
}

func NewDecapsulate(c net.Conn) net.Conn {
	rand.Seed(time.Now().UnixNano())
	return &Decapsulate{id: string(randBytes(10)), Conn: c, partionBuf: make([]byte, 0)}
}
