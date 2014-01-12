package main

import (
	"flag"
	"fmt"
)

var (
	shares = make(map[string]share) //map[secret]share
	port   = flag.Int("p", 6667, "Port to run on")
	sem    = make(chan int, MAX_CONNS)
	//local  = flag.Int("l", 3838, "Port for local broadcast")
)

const (
	BLOCK_SIZE   = 1 << 14 //2^14 = 16K
	PIECE_LENGTH = 1 << 18 //2^18 = 256K
	MAX_CONNS    = 64
)

//TODO stop littering this all over, go check actual errors
func check(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func init() {
	for i := 0; i < MAX_CONNS; i++ {
		sem <- 1
	}
}

func main() {
	flag.Parse()
	parseConfig()
	go sendMultiCast() //TODO more robust discovering
	incoming, outgoing := make(chan *UDPMessage), make(chan *UDPMessage)
	go listen(incoming)

	for {
		<-sem
		select {
		case m := <-incoming:
			go func(m *UDPMessage) {
				handleMessage(m, outgoing)
				sem <- 1
			}(m)
		case o := <-outgoing:
			go func(o *UDPMessage) {
				sendMessage(o)
				sem <- 1
			}(o)
		}
	}
}
