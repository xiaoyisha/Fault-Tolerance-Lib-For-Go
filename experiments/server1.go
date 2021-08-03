package main

import (
	perseus "Perseus"
	"fmt"
	"log"
	"net"
	"time"
)

var output1 chan bool
var errors1 chan error

// startServer1 starts the server1
func startServer1() {
	var i = 0
	listener, err := net.Listen("tcp", "localhost:8888")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	defer listener.Close()

	fmt.Println("server1 is listening......")
	for {
		i++
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleRequest1(conn, i)
	}
}

// connectServer12 server1 connects to the server2
func connectServer12() error {
	//time.Sleep(2*time.Second)
	conn, err := net.Dial("tcp", "localhost:8889")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	fmt.Println("connected successfully")
	cInfo := "Hello server2. I am server1."
	buffer := make([]byte, 2048)
	buffer = []byte(cInfo)

	// send information to the server
	_, err = conn.Write(buffer)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return err
	}
	// receive information from the server
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return err
	}
	fmt.Println(string(buffer))
	if len(errors1) == 0 {
		output1 <- true
	}
	return nil
}

// handleRequest1 the server1 handles a request from clients
func handleRequest1(conn net.Conn, num int) {
	fmt.Println("accept num:", num)
	clientInfo := make([]byte, 2048)
	// read the information from the client
	_, err := conn.Read(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}

	fmt.Println(string(clientInfo))
	sInfo := "Hello, I am server1."
	clientInfo = []byte(sInfo)
	// send the information to the client
	_, err = conn.Write(clientInfo)
	if err != nil {
		fmt.Println(" connection error: ", err)
		return
	}
	output1 = make(chan bool, 1)
	errors1 = make(chan error, 1)
	beforeRun := time.Now()
	fmt.Println("start time: ", beforeRun)
	errors1 = perseus.Go("my_command", connectServer12, nil)
	select {
	case _ = <-output1:
		// success
	case err := <-errors1:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration := time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)
}

func main() {
	startServer1()
}
