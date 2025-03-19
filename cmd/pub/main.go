package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/yhbsh/pubsub/pkg/client"
)

func main() {
	wg := sync.WaitGroup{}

	client := client.New(9000)
	for i := range 1000 {
		wg.Add(1)

		channel := fmt.Sprintf("channel_%d", i)
		payload := fmt.Sprintf("payload_%d", i)

		go func(channel string, payload string) {
			defer wg.Done()
			if err := client.Publish([]byte(channel), []byte(payload)); err != nil {
				log.Fatal(err)
			}

			log.Printf("Publish | channel -> %s | payload -> %s", channel, payload)
		}(channel, payload)
	}

	wg.Wait()
}
