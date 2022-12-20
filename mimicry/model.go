package mimicry

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

type Flow struct {
	Id       string  `json:"id"`
	Dst      string  `json:"dst"`
	Duration int64   `json:"duration"`
	Packs    []*Pack `json:"packs"`
}

type Pack struct {
	Length int   `json:"length"`
	Next   int64 `json:"next"`
}

func LoadFromFile(filename string) []Flow {
	f, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	r := bufio.NewReader(f)
	res := make([]Flow, 0)
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			panic(err)
		}
		if err == io.EOF {
			break
		}
		var flow Flow
		if err := json.Unmarshal([]byte(line), &flow); err != nil {
			panic(err)
		} else {
			res = append(res, flow)
		}
	}
	return res
}
