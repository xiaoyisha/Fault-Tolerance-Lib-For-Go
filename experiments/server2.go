package main

import (
	"fmt"
	"log"
	"net"
)

// startServer2 starts the server2
func startServer2() {
	var i = 0
	listener, err := net.Listen("tcp", "localhost:8889")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	defer listener.Close()

	fmt.Println("server2 is listening......")
	for {
		i++
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleRequest2(conn, i)
	}
}

// handleRequest2 the server2 handles a request from clients
func handleRequest2(conn net.Conn, num int) {
	fmt.Println("accept num:", num)
	clientInfo := make([]byte, 2048)
	// read the information from the client
	_, err := conn.Read(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}

	fmt.Println(string(clientInfo))
	sInfo := "Hello, I am server2."
	clientInfo = []byte(sInfo)
	// send the information to the client
	_, err = conn.Write(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}
}

func main() {
	startServer2()
}
