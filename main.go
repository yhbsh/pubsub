package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yhbsh/pubsub/pkg/server"
)

func main() {
    for i := 9000; i < 9020; i++ {
        server := server.New(i)
        go server.Run()
    }

	signalchannel := make(chan os.Signal, 1)
	signal.Notify(signalchannel, syscall.SIGINT, syscall.SIGTERM)
    <-signalchannel
    log.Print("Exiting ...")
}
