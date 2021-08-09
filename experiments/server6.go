package main

import (
	"fmt"
	"log"
	"net"
)

// startServer6 starts the server6
func startServer6() {
	var i = 0
	listener, err := net.Listen("tcp", "localhost:8891")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	defer listener.Close()

	fmt.Println("server6 is listening......")
	for {
		i++
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleRequest6(conn, i)
	}
}

// handleRequest6 the server6 handles a request from clients
func handleRequest6(conn net.Conn, num int) {
	fmt.Println("accept num:", num)
	clientInfo := make([]byte, 2048)
	// read the information from the client
	_, err := conn.Read(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}

	fmt.Println(string(clientInfo))
	sInfo := "Hello, I am server6."
	clientInfo = []byte(sInfo)
	// send the information to the client
	_, err = conn.Write(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}
}

func main() {
	startServer6()
}
