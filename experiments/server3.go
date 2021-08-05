package main

import (
	perseus "Perseus"
	pconfig "Perseus/config"
	"fmt"
	"log"
	"net"
	"time"
)

var output3 chan bool
var errors3 chan error

// connectServer34 server0 connects to the server1
func connectServer34() error {
	//time.Sleep(10 * time.Second)
	conn, err := net.Dial("tcp", "localhost:8890")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	fmt.Println("connected successfully")
	cInfo := "Hello server4. I am server3."
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
	return nil
}

func fallback3(err error) error {
	if err == perseus.ErrMaxConcurrency {
		log.Printf("fallBack func triggered by max concurrency")
		return nil
	}
	return err
}

func main() {
	var userMaxReq = 3
	var userCurReq = 4
	var orderMaxReq = 5
	var orderCurReq = 4
	for i := 0; i < userCurReq; i++ {
		output3 = make(chan bool, 1)
		errors3 = make(chan error, 1)
	}

	// run in the 'user' pool
	beforeRun := time.Now()
	fmt.Println("start time: ", beforeRun)

	pconfig.ConfigureCommand("user", pconfig.CommandConfig{
		MaxConcurrentRequests: userMaxReq,
	})
	for i := 0; i < userCurReq; i++ {
		errors3 = perseus.Go("user", func() error {
			err := connectServer34()
			if err != nil {
				log.Printf("run function err: %v", err)
				return err
			}
			output3 <- true
			return nil
		}, func(e error) error {
			err := fallback3(e)
			if err != nil {
				return err
			}
			output3 <- true
			return nil
		})
	}
	select {
	case _ = <-output3:
		// success
		log.Printf("success")
	case err := <-errors3:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration := time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)

	// run in the 'order' pool
	beforeRun = time.Now()
	fmt.Println("start time: ", beforeRun)

	pconfig.ConfigureCommand("order", pconfig.CommandConfig{
		MaxConcurrentRequests: orderMaxReq,
	})
	for i := 0; i < orderCurReq; i++ {
		errors3 = perseus.Go("order", func() error {
			err := connectServer34()
			if err != nil {
				log.Printf("run function err: %v", err)
				return err
			}
			output3 <- true
			return nil
		}, func(e error) error {
			err := fallback3(e)
			if err != nil {
				return err
			}
			output3 <- true
			return nil
		})
	}
	select {
	case _ = <-output3:
		// success
		log.Printf("success")
	case err := <-errors3:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration = time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)
}
