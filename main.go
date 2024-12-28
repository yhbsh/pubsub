package main

import (
	"crypto/tls"
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
)

type Message struct {
	Channel string
	Payload []byte
}

type Subscription struct {
	conn     net.Conn
	channels map[string]struct{}
}

type PubSub struct {
	subscribers map[net.Conn]*Subscription
	mu          sync.RWMutex
	msgChan     chan Message
}

func newPubSub() *PubSub {
	return &PubSub{
		subscribers: make(map[net.Conn]*Subscription),
		msgChan:     make(chan Message),
	}
}

func (pubsub *PubSub) removeConn(conn net.Conn) {
	pubsub.mu.Lock()
	defer pubsub.mu.Unlock()
	delete(pubsub.subscribers, conn)
}

func (pubsub *PubSub) subscribe(conn net.Conn, channel string) {
	pubsub.mu.Lock()
	defer pubsub.mu.Unlock()

	sub, exists := pubsub.subscribers[conn]
	if !exists {
		sub = &Subscription{conn: conn, channels: make(map[string]struct{})}
		pubsub.subscribers[conn] = sub
	}
	sub.channels[channel] = struct{}{} // Add channel to subscription
}

func (pubsub *PubSub) unsubscribe(conn net.Conn, channel string) {
	pubsub.mu.Lock()
	defer pubsub.mu.Unlock()

	if sub, exists := pubsub.subscribers[conn]; exists {
		delete(sub.channels, channel)
		// Remove subscriber if they're not subscribed to any channels
		if len(sub.channels) == 0 {
			delete(pubsub.subscribers, conn)
		}
	}
}

func (pubsub *PubSub) broadcast() {
	for msg := range pubsub.msgChan {
		pubsub.mu.RLock()
		for _, sub := range pubsub.subscribers {
			if _, isSubscribed := sub.channels[msg.Channel]; isSubscribed {
				go func(conn net.Conn, message Message) {
					channelLen := uint8(len(message.Channel))
					payloadLen := uint32(len(message.Payload))
					totalLen := 1 + int(channelLen) + 4 + len(message.Payload)
					buf := make([]byte, totalLen)

					buf[0] = channelLen
					copy(buf[1:], message.Channel)
					binary.LittleEndian.PutUint32(buf[1+channelLen:], payloadLen)
					copy(buf[1+channelLen+4:], message.Payload)

					n, err := conn.Write(buf)
					if err != nil || n != totalLen {
						pubsub.removeConn(conn)
						log.Println(err)
						return
					}
				}(sub.conn, msg)
			}
		}
		pubsub.mu.RUnlock()
	}
}

func (pubsub *PubSub) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("[client %s] connected", conn.RemoteAddr())
	defer log.Printf("[client %s] disconnected", conn.RemoteAddr())

	for {
		var cmdType uint8
		if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
			if err != io.EOF {
				log.Println(err)
			}
			pubsub.removeConn(conn)
			return
		}

		switch cmdType {
		case 0x00: // Publish
			var channelLen uint8
			if err := binary.Read(conn, binary.LittleEndian, &channelLen); err != nil {
				log.Println(err)
				return
			}

			channel := make([]byte, channelLen)
			if n, err := io.ReadFull(conn, channel); err != nil {
				log.Println(err)
				return
			} else if n != int(channelLen) {
				log.Println("invalid channel length")
				return
			}

			var payloadLen uint32
			if err := binary.Read(conn, binary.LittleEndian, &payloadLen); err != nil {
				log.Println(err)
				return
			}

			payload := make([]byte, payloadLen)
			if n, err := io.ReadFull(conn, payload); err != nil {
				log.Println(err)
				return
			} else if n != int(payloadLen) {
				log.Println("invalid payload length")
				return
			}

			pubsub.msgChan <- Message{Channel: string(channel), Payload: payload}

		case 0x01: // Subscribe
			var channelLen uint8
			if err := binary.Read(conn, binary.LittleEndian, &channelLen); err != nil {
				log.Println(err)
				return
			}

			channel := make([]byte, channelLen)
			if n, err := io.ReadFull(conn, channel); err != nil {
				log.Println(err)
				return
			} else if n != int(channelLen) {
				log.Println("invalid channel length")
				return
			}

			pubsub.subscribe(conn, string(channel))

		case 0x02: // Unsubscribe
			var channelLen uint8
			if err := binary.Read(conn, binary.LittleEndian, &channelLen); err != nil {
				log.Println(err)
				return
			}

			channel := make([]byte, channelLen)
			if n, err := io.ReadFull(conn, channel); err != nil {
				log.Println(err)
				return
			} else if n != int(channelLen) {
				log.Println("invalid channel length")
				return
			}

			pubsub.unsubscribe(conn, string(channel))
		}
	}
}

func main() {
	pubsub := newPubSub()
	go pubsub.broadcast()

	ln, err := net.Listen("tcp", "0.0.0.0:8081")
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("Listening on port 8081")

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go pubsub.handleConnection(conn)
	}
}
