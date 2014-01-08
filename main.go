package main

import (
	"flag"
	"fmt"
)

var (
	shares = make(map[string]share) //map[secret]share
	port   = flag.Int("p", 6667, "Port to run on")
	//local  = flag.Int("l", 3838, "Port for local broadcast")
)

const (
	BLOCK_SIZE   = 1 << 14 //2^14 = 16K
	PIECE_LENGTH = 1 << 18 //2^18 = 256K
)

func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	//discover motha fuckas
	flag.Parse()
	parseConfig()
	go sendMultiCast()
	incoming := make(chan UDPMessage)
	listen(incoming)
	outgoing := make(chan UDPMessage)

	for {
		select {
		case m := <-incoming:
			go handleMessage(m, outgoing)
		case o := <-outgoing:
			go sendMessage(o)
		}
	}
}
