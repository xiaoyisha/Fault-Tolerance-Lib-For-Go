package main

import (
	perseus "Perseus"
	pconfig "Perseus/config"
	"fmt"
	"log"
	"net"
	"time"
)

var output5 chan bool
var errors5 chan error
var output51 chan bool
var errors51 chan error
var output52 chan bool
var errors52 chan error

// connectServer57 server5 connects to the server7
func connectServer57() error {
	//time.Sleep(10 * time.Second)
	conn, err := net.Dial("tcp", "localhost:8891")
	if err != nil {
		log.Fatal("an error!", err.Error())
	}
	fmt.Println("connected successfully")
	cInfo := "Hello server7. I am server5."
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

func main() {
	var orderCurReq = 8
	output5 = make(chan bool, 1)
	errors5 = make(chan error, 1)
	output51 = make(chan bool, 1)
	errors51 = make(chan error, 1)
	output52 = make(chan bool, 1)
	errors52 = make(chan error, 1)

	// run in the 'order' pool
	beforeRun := time.Now()
	fmt.Println("start time: ", beforeRun)

	pconfig.ConfigureCommand("order", pconfig.CommandConfig{
		RequestVolumeThreshold: 6,
	})
	for i := 0; i < orderCurReq; i++ {
		errors5 = perseus.Go("order", func() error {
			err := connectServer57()
			if err != nil {
				log.Printf("run function err: %v", err)
				return err
			}
			output5 <- true
			return nil
		}, nil)
		// metricCollector needs time to collect data
		time.Sleep(1 * time.Second)
	}
	select {
	case _ = <-output5:
		// success
		log.Printf("success")
	case err := <-errors5:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration := time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)

	// run in the 'user' pool
	beforeRun = time.Now()
	fmt.Println("start time: ", beforeRun)
	errors51 = perseus.Go("user", func() error {
		err := connectServer57()
		if err != nil {
			log.Printf("run function err: %v", err)
			return err
		}
		output51 <- true
		return nil
	}, nil)
	select {
	case _ = <-output51:
		// success
		log.Printf("success")
	case err := <-errors51:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration = time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)

	// run in the 'order' pool
	// sleep window of testing circuits is 5 second
	time.Sleep(5 * time.Second)
	beforeRun = time.Now()
	fmt.Println("start time: ", beforeRun)
	errors52 = perseus.Go("order", func() error {
		err := connectServer57()
		if err != nil {
			log.Printf("run function err: %v", err)
			return err
		}
		output52 <- true
		return nil
	}, nil)
	select {
	case _ = <-output52:
		// success
		log.Printf("success")
	case err := <-errors52:
		// failure
		log.Printf("err: %v", err)
	}

	// get the running duration
	duration = time.Since(beforeRun)
	fmt.Println("Running duration: ", duration)

}
