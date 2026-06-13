package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("Dialing 127.0.0.1:9090...")
	conn, err := net.DialTimeout("tcp", "127.0.0.1:9090", 5*time.Second)
	if err != nil {
		fmt.Printf("Dial failed: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected successfully! Writing data...")
	_, err = conn.Write([]byte("Hello\n"))
	if err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	fmt.Println("Reading response...")
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}

	fmt.Printf("Response received: %q\n", resp)
}
