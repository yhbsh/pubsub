package main

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"sync"
)

type message struct {
	Channel string
	Payload []byte
}

type pubSub struct {
	channels map[string][]net.Conn
	mu       sync.RWMutex
	msgChan  chan message
}

func newPubSub() *pubSub {
	return &pubSub{
		channels: make(map[string][]net.Conn),
		msgChan:  make(chan message, 100),
	}
}

func (s *pubSub) removeConn(channel string, conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conns, ok := s.channels[channel]; ok {
		for i, c := range conns {
			if c == conn {
				s.channels[channel] = append(conns[:i], conns[i+1:]...)
				break
			}
		}

		if len(s.channels[channel]) == 0 {
			delete(s.channels, channel)
		}
	}
}

func (s *pubSub) broadcast() {
	for msg := range s.msgChan {
		s.mu.RLock()
		if clients, ok := s.channels[msg.Channel]; ok {
			for i, client := range clients {
				go func(i int, client net.Conn, payload []byte) {
					if _, err := client.Write(payload); err != nil {
						log.Printf("cannot write to client %d because of %v", i, err)
						s.removeConn(msg.Channel, client)
						return
					}
				}(i, client, msg.Payload)
			}
		}
		s.mu.RUnlock()
	}
}

func (s *pubSub) handlePublish(conn net.Conn) error {
	var channelLen uint8
	if err := binary.Read(conn, binary.LittleEndian, &channelLen); err != nil {
		return err
	}

	channel := make([]byte, channelLen)
	if n, err := io.ReadFull(conn, channel); err != nil {
		return err
	} else if n != int(channelLen) {
		return io.ErrShortBuffer
	}

	var payloadLen uint32
	if err := binary.Read(conn, binary.LittleEndian, &payloadLen); err != nil {
		return err
	}

	payload := make([]byte, payloadLen)
	if n, err := io.ReadFull(conn, payload); err != nil {
		return err
	} else if n != int(payloadLen) {
		return io.ErrShortBuffer
	}

	s.msgChan <- message{
		Channel: string(channel),
		Payload: payload,
	}
	return nil
}

func (s *pubSub) handleSubscribe(conn net.Conn) error {
	var channelLen uint8
	if err := binary.Read(conn, binary.LittleEndian, &channelLen); err != nil {
		return err
	}

	channel := make([]byte, channelLen)
	if n, err := io.ReadFull(conn, channel); err != nil {
		return err
	} else if n != int(channelLen) {
		return io.ErrShortBuffer
	}

	channelStr := string(channel)

	s.mu.Lock()
	s.channels[channelStr] = append(s.channels[channelStr], conn)
	s.mu.Unlock()

	buf := make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			s.removeConn(channelStr, conn)
			return err
		}
	}
}

func (s *pubSub) handleConnection(conn net.Conn) {
	defer conn.Close()

	log.Printf("[client %s] connected", conn.RemoteAddr())
	defer log.Printf("[client %s] disconnected", conn.RemoteAddr())

	var cmdType uint8

	for {
		if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
			if err != io.EOF {
				log.Println(err)
			}
			return
		}
		if cmdType == 0x00 {
			if err := s.handlePublish(conn); err != nil {
				log.Println(err)
				return
			}
		} else {
			break
		}

	}

	if cmdType == 0x01 {
		if err := s.handleSubscribe(conn); err != nil {
			return
		}
	} else {
		log.Println("invalid command type")
		return
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
