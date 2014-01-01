package main

import (
	"fmt"
)

var (
	shares = make(map[string]share) //map[secret]share
	port   = 6667
)

const (
	BLOCK_SIZE   = 1 << 14 //2^14
	PIECE_LENGTH = 1 << 18 //2^18
)

func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	//discover motha fuckas
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
