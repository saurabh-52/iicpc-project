package main

import (
    "bufio"
    "fmt"
    "net"
)

func main() {
    ln, err := net.Listen("tcp", ":9090")
    if err != nil {
        fmt.Println("listen error:", err)
        return
    }
    fmt.Println("TCP server listening on :9090")
    for {
        conn, err := ln.Accept()
        if err != nil {
            continue
        }
        go func(c net.Conn) {
            defer c.Close()
            r := bufio.NewReader(c)
            for {
                line, err := r.ReadBytes('\n')
                if err != nil {
                    return
                }
                // simple ack
                c.Write([]byte("OK\n"))
                _ = line
            }
        }(conn)
    }
}
