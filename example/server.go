package main

import (
	"fmt"
	"log"
	"net"
)

// startServer starts the server
func startServer() {
	var i = 0
	listener, err := net.Listen("tcp", "localhost:8888")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	defer listener.Close()

	fmt.Println("the server is listening")
	for {
		i++
		fmt.Println("before accept")
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleRequest(conn, i)
	}
}

// handleRequest the server handles a request from clients
func handleRequest(conn net.Conn, num int) {
	fmt.Println("accept num:", num)
	clientInfo := make([]byte, 2048)
	// read the information from the client
	_, err := conn.Read(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}

	fmt.Println(string(clientInfo))
	sInfo := "hello, I am server"
	clientInfo = []byte(sInfo)
	// send the information to the client
	_, err = conn.Write(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}
}

func main() {
	startServer()
}
