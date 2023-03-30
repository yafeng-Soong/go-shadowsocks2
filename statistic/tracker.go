package statistic

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	"github.com/gofrs/uuid"
)

const logFile = "ss_statistic.log"

type Packet struct {
	Real_bytes  int `json:"real_bytes"`
	Total_bytes int `json:"total_bytes"`
}

type Metadata struct {
	SrcIP   net.IP `json:"source_IP"`
	DstIP   net.IP `json:"destination_IP"`
	SrcPort string `json:"source_port"`
	DstPort string `json:"destination_port"`
	Host    string `json:"destination_host"`
}

func (m *Metadata) LocalAndRemoteAddress() string {
	return strings.Join([]string{m.SrcIP.String(), m.DstString(), m.DstPort}, "-")
}

func (m *Metadata) RemoteAddress() string {
	return net.JoinHostPort(m.DstString(), m.DstPort)
}

func (m *Metadata) SourceAddress() string {
	return net.JoinHostPort(m.SrcIP.String(), m.SrcPort)
}

func (m *Metadata) DstString() string {
	if m.Host != "" {
		return m.Host
	} else if m.DstIP != nil {
		return m.DstIP.String()
	} else {
		return "<nil>"
	}
}

type TrackerInfo struct {
	UUID           uuid.UUID `json:"id"`
	Metadata       *Metadata `json:"metadata"`
	Start          time.Time `json:"-"`
	End            time.Time `json:"-"`
	StartTime      int64     `json:"start_time"`
	EndTime        int64     `json:"end_time"`
	UploadReal     int       `json:"upload_real"`
	UploadTotal    int       `json:"upload_total"`
	UploadPure     int       `json:"upload_pure"`
	UploadPurePlus int       `json:"upload_pure_plus"`
	UploadNums     int       `json:"upload_nums"`
	UploadPackets  []*Packet `json:"upload_packets"`
	Error          bool      `json:"error"` // 连接是否被意外中断
}

// 从chan中读取每条的统计信息
func HandleMetric(ch <-chan *TrackerInfo) {
	for info := range ch {
		outpath := defaultPath()
		if ok, _ := PathExists(outpath); !ok {
			err := os.MkdirAll(outpath, os.ModePerm)
			if err != nil {
				log.Println("创建文件夹", outpath, "出错")
				continue
			}
		}
		filename := path.Join(outpath, logFile)
		writeToFile(filename, info)
	}
}

func writeToFile(filename string, info *TrackerInfo) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		log.Println("打开文件", filename, "出错")
		return
	}
	defer f.Close()
	b, err := json.Marshal(info)
	if err != nil {
		log.Println("序列化", info.UUID, "出错")
		return
	}
	_, err = f.Write(b)
	if err != nil {
		log.Println("写入文件", filename, "出错")
		return
	}
	f.WriteString("\n")
	log.Println(info.UUID, "记录完成")
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func defaultPath() string {
	u, err := user.Current()
	if err != nil {
		log.Println("获取用户出错")
		return "./"
	}
	return path.Join(u.HomeDir, "logs")
}
