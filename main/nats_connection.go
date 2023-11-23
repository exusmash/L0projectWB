package main

import (
	"github.com/nats-io/stan.go"
	"log"
	"time"
)

const (
	natsURL   = "nats://localhost:4222"
	clusterID = "test-cluster"
	clientID  = "service-client"
	subject   = "new-orders"
)

var sc stan.Conn

func initNATS() {
	var err error
	for {
		// Попытка подключения к NATS-streaming с ретраями в случае неудачи
		sc, err = stan.Connect(clusterID, clientID, stan.NatsURL(natsURL))
		if err == nil {
			break
		}
		log.Printf("Не удалось подключиться к NATS: %v, повторная попытка через 5 секунд", err)
		time.Sleep(5 * time.Second)
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
			log.Println(err)
		}
	}
}
