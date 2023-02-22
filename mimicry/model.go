package mimicry

import (
	"container/list"
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	pb "github.com/yafeng-Soong/go-shadowsocks2/mimicry/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	address = "127.0.0.1:50051"
)

// 持有从rpc请求到的Flow集合
type FlowContainer struct {
	flows     *list.List
	lock      sync.Mutex
	conn      *grpc.ClientConn
	isRequest int32
	client    pb.GrpcServiceClient
}

// 获取一条flow
func (fc *FlowContainer) GetFlow() *pb.Flow {
	fc.lock.Lock()
	defer fc.lock.Unlock()
	flow := fc.flows.Remove(fc.flows.Front()).(*pb.Flow)
	if fc.flows.Len() < 50 {
		go fc.requestFlow()
	}
	return flow
}

// 补充flows
func (fc *FlowContainer) requestFlow() {
	if atomic.CompareAndSwapInt32(&fc.isRequest, 0, 1) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		log.Println("flows不足，开始补充flows")
		res, err := fc.client.Flows(ctx, &empty.Empty{})
		if err != nil {
			log.Println("补充flows出错")
		} else {
			fc.lock.Lock()
			defer fc.lock.Unlock()
			for _, flow := range res.Flows {
				fc.flows.PushBack(flow)
			}
			log.Println("补充flows完毕，还剩", fc.flows.Len())
		}
		atomic.StoreInt32(&fc.isRequest, 0)
	}
}

// 从rpc请求flows
func NewFlowContainer() *FlowContainer {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("can't connect to: %s", address)
	}
	client := pb.NewGrpcServiceClient(conn)
	flows := list.New()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	res, err := client.Flows(ctx, &empty.Empty{})
	if err != nil {
		log.Fatalln("初始化请求rpc出错")
	}
	for _, flow := range res.Flows {
		flows.PushBack(flow)
	}
	return &FlowContainer{
		flows:     flows,
		isRequest: 0,
		conn:      conn,
		client:    client,
	}
}
