package main

import (
	perseus "Perseus"
	"fmt"
	"log"
	"net"
	"time"
)

var output0 chan bool
var errors0 chan error

// connectServer01 server0 connects to the server1
func connectServer01() error {
	conn, err := net.Dial("tcp", "localhost:8888")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	fmt.Println("connected successfully")
	cInfo := "Hello server1. I am server0."
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
	output0 <- true
	return nil
}

func fallback0(err error) error {
	if err == perseus.ErrTimeout {
		log.Printf("fallBack func triggered by timeout")
		output0 <- true
		return nil
	}
	return err
}

func main() {
	output0 = make(chan bool, 1)
	errors0 = make(chan error, 1)
	beforeRun := time.Now()
	fmt.Println("start time: ", beforeRun)

	// with Perseus
	errors0 = perseus.Go("my_command", connectServer01, fallback0)
	select {
	case _ = <-output0:
		// success
		log.Printf("success")
	case err := <-errors0:
		// failure
		log.Printf("err: %v", err)
	}

	// without Perseus
	//err := connectServer01()
	//if err != nil {
	//	log.Printf("err: %v", err)
	//}

	// get the running duration
	duration := time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)
}
