package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"
)

var protocol string
var address string
var timeout string

func init() {
	flag.StringVar(&protocol, "protocol", "tcp", "protocol")
	flag.StringVar(&address, "address", "", "address")
	flag.StringVar(&timeout, "timeout", "10s", "connection timeout")
}

// Reader goroutine
func Reader(ctx context.Context, conn net.Conn) {
EXITIN:
	for {
		select {
		case <-ctx.Done():
			break EXITIN
		default:
			timeoutDuration := time.Second * 10
			bufReader := bufio.NewReader(conn)

			for {
				err := conn.SetReadDeadline(time.Now().Add(timeoutDuration))
				if err != nil {
					log.Printf("unable to set ReadDeadline timeout: %v", err)
				}

				bytes, err := bufReader.ReadBytes('\n')
				if err != nil {
					continue EXITIN
				}

				log.Printf("from server: %s", bytes)
			}
		}
	}
}

// Writer goroutine
func Writer(ctx context.Context, conn net.Conn) {
	input := make(chan string, 2)
	go getInput(input)
EXITOUT:
	for {
		select {
		case <-ctx.Done():
			break EXITOUT
		case i := <-input:
			log.Printf("%v\n", i)
			if i == io.EOF.Error() {
				ctx.Done()
			}
			_, err := conn.Write([]byte(fmt.Sprintf("%s\n", i)))
			if err != nil {
				ctx.Done()
				log.Printf("cannot write to socket: %v", err)
			}
		}
	}
}

func getInput(input chan string) {
	for {
		in := bufio.NewReader(os.Stdin)
		result, err := in.ReadString('\n')
		if err != nil || err == io.EOF {
			input <- err.Error()
			log.Printf("unexpected end of file: %v", err)
		}
		input <- result
	}
}

func main() {
	flag.Parse()
	dialer := &net.Dialer{}
	ctx := context.Background()
	timeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Fatalf("Unable to parse timeout: %v", err)
	}
	timeoutResult := timeout.Seconds()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutResult)*time.Second)
	address = flag.Arg(0) + ":" + flag.Arg(1)
	conn, err := dialer.DialContext(ctx, protocol, address)

	if err != nil {
		log.Fatalf("Unable to connect: %v", err)
	}

	wg := sync.WaitGroup{}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c
		log.Fatalf("keyboard interrupt")
	}()

	wg.Add(1)

	go func() {
		Reader(ctx, conn)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		Writer(ctx, conn)
		wg.Done()
	}()

	wg.Wait()

	cancel()
	err = conn.Close()
	if err != nil {
		log.Fatal("Connection was closed", err)
	}
}
