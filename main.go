package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type Args struct {
	inPort  int
	outPort int
	outIP   string
}

func getArgs() Args {
	inPortPtr := flag.Int("in-port", 0, "The in port on which to listen")
	outPortPtr := flag.Int("out-port", 0, "The out port to which to output")
	outIPPtr := flag.String("out-ip", "127.0.0.1", "The out ip to which to output")
	flag.Parse()

	if *inPortPtr == 0 || *outPortPtr == 0 {
		fmt.Printf("%v: Both -in and -out ports must be specified\n", strings.Join(os.Args, " "))
		fmt.Println("Run with -help for usage.")

		os.Exit(1)
	}

	return Args{*inPortPtr, *outPortPtr, *outIPPtr}
}

func forward(source io.Reader, dest io.Writer) {
	buffer := make([]byte, 1<<16)
	for {
		size, err := source.Read(buffer)
		if err != nil {
			log.Fatalf("Read failed: %v", err)
		}

		_, err = dest.Write(buffer[:size])
		if err != nil {
			log.Fatalf("Write failed: %v", err)
		}
	}
}

func main() {
	args := getArgs()

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", args.inPort))
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func(inConn net.Conn) {
			var dialer net.Dialer

			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			outConn, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", args.outIP, args.outPort))
			if err != nil {
				log.Fatalf("Failed to dial: %v", err)
			}

			go forward(inConn, outConn)
			go forward(outConn, inConn)
		}(conn)
	}
}
