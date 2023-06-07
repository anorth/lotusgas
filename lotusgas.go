package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type Tally struct {
	From     string
	To       string
	Depth    int
	Method   uint64
	SelfGas  uint64
	TotalGas uint64
}

// Analyses a JSON object containing a Lotus execution trace to determine the self and total gas consumption
// of each message execution in the call tree.
//
// Usage: go run github.com/anorth/lotusgas --depth 2 <json-filename>
func main() {
	displayDepth := flag.Int("depth", math.MaxInt32, "max depth to display")
	flag.Parse()
	inPath := flag.Arg(0)

	full := Load(inPath)
	topLevelMsgs := full["Trace"].([]interface{})
	_, _ = fmt.Printf("call\tself\ttotal\n") // TSV header row
	for _, topMsg := range topLevelMsgs {
		topMsg := topMsg.(map[string]interface{})
		label := topMsg["MsgCid"].(map[string]interface{})["/"]
		trace := topMsg["ExecutionTrace"].(map[string]interface{})
		tallies := TallyCalls(trace, 0)
		totalGas := uint64(0)
		p := message.NewPrinter(language.English) // Pretty-print big numbers with thousands separators.
		_, _ = p.Printf("%s\n", label)
		for _, r := range tallies {
			totalGas += r.SelfGas
			if r.Depth < *displayDepth {
				indent := strings.Repeat("  ", r.Depth)
				method := fmt.Sprintf("%d", r.Method) // Don't group thousands in method number
				_, _ = p.Printf("%s%s→%s:%s\t%12v\t%12v\n", indent, r.From, r.To, method, r.SelfGas, r.TotalGas)
			}
		}
	}
}

// Reads and unmarshals the JSON into memory.
func Load(path string) map[string]interface{} {
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	var dat map[string]interface{}
	if err := json.Unmarshal(raw, &dat); err != nil {
		panic(err)
	}
	return dat
}

// Tallys the gas consumption of a message execution and its subcalls.
// Returns a slice of tallys, in call sequence.
func TallyCalls(trace map[string]interface{}, depth int) []Tally {
	msg := trace["Msg"].(map[string]interface{})
	charges := trace["GasCharges"].([]interface{})
	selfGas := float64(0)
	for _, charge := range charges {
		selfGas += charge.(map[string]interface{})["tg"].(float64)
	}

	result := []Tally{{
		From:     msg["From"].(string),
		To:       msg["To"].(string),
		Depth:    depth,
		Method:   uint64(msg["Method"].(float64)),
		SelfGas:  uint64(selfGas),
		TotalGas: uint64(selfGas),
	}}

	subcalls := trace["Subcalls"]
	totalGas := uint64(0)
	if subcalls != nil {
		for _, call := range subcalls.([]interface{}) {
			subResult := TallyCalls(call.(map[string]interface{}), depth+1)
			result = append(result, subResult...)
			totalGas += subResult[0].TotalGas
		}
	}
	result[0].TotalGas += totalGas
	return result
}
