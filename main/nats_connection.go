package main

import (
	_ "fmt"
	"github.com/nats-io/stan.go"
	"log"
	_ "time"
)

const (
	natsURL   = "nats://localhost:4222"
	clusterID = "test-cluster"
	clientID  = "service-client"
	channel   = "orders"
	durableID = "service-durable"
	subject   = "new-orders"
)

var sc stan.Conn

func initNATS() {
	var err error
	//Подключение к NATS-streaming
	sc, err = stan.Connect(clusterID, clientID, stan.NatsURL(natsURL))
	if err != nil {
		log.Fatal(err)
	}
}

func subscribeToOrders(handleMessageFunc func([]byte)) stan.Subscription {
	subscription, err := sc.Subscribe(subject, func(msg *stan.Msg) {
		handleMessageFunc(msg.Data)
	})
	if err != nil {
		log.Fatal(err)
	}
	return subscription
}

func closeNATS() {
	if sc != nil {
		err := sc.Close()
		if err != nil {
			return
		}
	}
}
