package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yhbsh/pubsub/pkg/server"
)

func main() {
	server := server.New(9000)
	go server.Run()

	signalchannel := make(chan os.Signal, 1)
	signal.Notify(signalchannel, syscall.SIGINT, syscall.SIGTERM)
	<-signalchannel
	log.Print("Exiting ...")
}
