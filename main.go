package main

import (
	"encoding/json"
	"flag"
	"log"
	"strconv"
	"strings"
)

// syntax: go run main.go <ip>:<port> -t <timeout> -c <count>

func main() {
	// Parse the command line flags
	flag.Parse()

	// get address from command line
	ipport := flag.Arg(0)
	// get timeout from command line
	timeout := flag.Int("t", 5, "timeout")
	// get count from command line
	count := flag.Int("c", 3, "count")

	var ip string

	var portStr string
	if i := strings.LastIndex(ipport, ":"); i > 0 {
		portStr = ipport[i+1:]
		ip = ipport[:i]
	} else {
		log.Println("Invalid address")
		return
	}

	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil {
		panic(err)
	}

	params := params{
		Destination:     ip,
		DestinationPort: int(port),
		Timeout:         *timeout,
		Count:           *count,
		Protocol:        "udp",
	}
	res := make([]result, *count)
	// new runner
	r := &run{
		Parameters: params,
		Results:    res,
	}

	// run
	if err := r.Run(); err != nil {
		panic(err)
	}

	// print results
	println(prettyPrint(r.Results))

}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}
