/*
	14_fastjson shows reflection-free JSON marshaling with strconv.Append*.

$ go run main.go
*/
package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type Metric struct {
	Name  string
	Value int64
}

// AppendJSON builds the JSON directly into dst - no reflection, no fmt.
func (m Metric) AppendJSON(dst []byte) []byte {
	dst = append(dst, `{"name":`...)
	dst = strconv.AppendQuote(dst, m.Name)
	dst = append(dst, `,"value":`...)
	dst = strconv.AppendInt(dst, m.Value, 10)
	dst = append(dst, '}')
	return dst
}

func main() {
	m := Metric{Name: "cpu.usage", Value: 42}

	fast := m.AppendJSON(make([]byte, 0, 64))
	standard, _ := json.Marshal(m)

	fmt.Printf("hand-rolled: %s\n", fast)
	fmt.Printf("encoding/json: %s\n", standard)
}
