package main

import (
	lib "Fault-Tolerance-Lib-For-Go"
	"fmt"
	"log"
	"net"
	"time"
)

var output chan bool
var errors chan error

// connectServer the client connects to the server
func connectServer() error {
	conn, err := net.Dial("tcp", "localhost:8888")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	fmt.Println("connected successfully\n")

	cInfo := "Hello.I am client     "
	buffer := make([]byte, 2048)
	buffer = []byte(cInfo)

	conn.Write(buffer)
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
	output <- true
	return nil
}

func main() {
	var goCount = 3
	output = make(chan bool, goCount)
	errors = make(chan error, goCount)
	tInsert := time.Now()
	fmt.Println("tStart time: ", tInsert)
	for i := 0; i < goCount; i++ {
		fmt.Println("goroutine number: ", i)
		errors = lib.Go("my_command", connectServer, nil)
	}
	for i := 0; i < goCount; i++ {
		select {
		case _ = <-output:
			// success
		case err := <-errors:
			// failure
			log.Printf("goroutine number: %v, err: %v", i, err)
		}
	}

	// get the running duration
	elapsed := time.Since(tInsert)
	fmt.Println("Insert elapsed: ", elapsed)
	fmt.Println("/n")
}
