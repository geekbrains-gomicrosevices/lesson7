package main

import (
	"fmt"
	"github.com/marpaia/graphite-golang"
	"gopkg.in/alexcesaro/statsd.v2"
	"log"
	"math/rand"
	"time"
)

func main() {
	// go sendGraphite()
	go sendStatsD()
	select {}
}

func sendStatsD() {
	c, err := statsd.New()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 1; ; i++ {
		log.Print("Send")
		c.Increment("foo.counter")
		time.Sleep(time.Duration(r.Intn(1000)) * time.Millisecond)
	}
}

func sendGraphite() {
	g, e := graphite.NewGraphite("localhost", 2013)
	if e != nil {
		log.Fatal(e)
	}

	for i := 1; ; i++ {
		g.SimpleSend("test1.test1", fmt.Sprintf("%v", i))
		if i == 20 {
			i = 0
		}
		log.Print("send grapnite")
		time.Sleep(time.Second)
	}
}
