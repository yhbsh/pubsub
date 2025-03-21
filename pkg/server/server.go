package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/yhbsh/pubsub/pkg/utils"
)

type Server struct {
	channels map[string]map[net.Conn]struct{}
	mutex    *sync.RWMutex
	port     int
}

func New(port int) *Server {
	channels := make(map[string]map[net.Conn]struct{}, 500)
	mutex := &sync.RWMutex{}
	return &Server{channels: channels, mutex: mutex, port: port}
}

func (server *Server) runTcp() error {
	addr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("0.0.0.0:%d", server.port))
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenTCP("tcp4", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	log.Printf("Listening on addr %s (tcp)", addr)

	for {
		conn, err := ln.AcceptTCP()
		if err != nil {
			log.Printf("Accept error %v", err)
			continue
		}

		go server.serveSub(conn)
	}
}

func (server *Server) runUnix() error {
	path := fmt.Sprintf("/tmp/.s.PUBSUB.%d", server.port)
	if err := os.RemoveAll(path); err != nil {
		log.Fatal(err)
	}

	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.ListenUnix("unix", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	log.Printf("Listening on path %s (unix)", path)

	for {
		conn, err := ln.AcceptUnix()
		if err != nil {
			log.Print("Accept error:", err)
			continue
		}

		go server.servePub(conn)
	}
}

func (server *Server) Run() {
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		server.runTcp()
	}()

	go func() {
		defer wg.Done()
		server.runUnix()
	}()

	wg.Wait()
}

func (server *Server) serveSub(conn net.Conn) {
	log.Printf("[conn %s] connected", conn.RemoteAddr())

	defer func() {
		log.Printf("[conn %s] disconnected", conn.RemoteAddr())
		conn.Close()

		server.mutex.Lock()
		defer server.mutex.Unlock()
		for channel := range server.channels {
			delete(server.channels[channel], conn)
		}
	}()

	for {
		var commandType uint8
		if err := binary.Read(conn, binary.LittleEndian, &commandType); err != nil {
			log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
			return
		}

		channel, err := utils.R8le(conn)
		if err != nil {
			log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
			return
		}

		switch commandType {
		case 1:
			server.subscribe(conn, string(channel))
		case 2:
			server.unsubscribe(conn, string(channel))
		default:
			log.Printf("[conn %s] invalid command type", conn.RemoteAddr())
		}
	}
}

func (server *Server) servePub(conn net.Conn) {
	defer conn.Close()

	var cmdType uint8
	if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
		log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
		return
	}

	if cmdType != 0 {
		log.Printf("invalid command type from publisher")
		return
	}

	channel, err := utils.R8le(conn)
	if err != nil {
		log.Print(err)
		return
	}

	payload, err := utils.R32le(conn)
	if err != nil {
		log.Print(err)
		return
	}

	go server.publish(channel, payload)
}

func (server *Server) subscribe(conn net.Conn, channel string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	if _, ok := server.channels[channel]; !ok {
		server.channels[channel] = make(map[net.Conn]struct{})
	}

	server.channels[channel][conn] = struct{}{}
	log.Printf("[conn %s] [subscribed %s]", conn.RemoteAddr(), string(channel))
}

func (server *Server) unsubscribe(conn net.Conn, channel string) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	if clients, ok := server.channels[channel]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			delete(server.channels, channel)
		}
		log.Printf("[conn %s] [unsubscribed %s]", conn.RemoteAddr(), string(channel))
	}
}

func (server *Server) publish(channel []byte, payload []byte) {
	server.mutex.Lock()
	conns, exists := server.channels[string(channel)]
	server.mutex.Unlock()

	if !exists {
		log.Printf("[channel %s] no publish, no clients", channel)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(conns))

	for conn := range conns {
		go func(conn net.Conn, channel []byte, payload []byte) {
			defer wg.Done()

			var buf bytes.Buffer
			buf.WriteByte(byte(len(channel)))
			buf.Write(channel)
			buf.WriteByte(byte(len(payload) >> (8 * 0)))
			buf.WriteByte(byte(len(payload) >> (8 * 1)))
			buf.WriteByte(byte(len(payload) >> (8 * 2)))
			buf.WriteByte(byte(len(payload) >> (8 * 3)))
			buf.Write(payload)

			log.Printf("[channel %s] publish... client %s", channel, conn.RemoteAddr())
			utils.DumpJson(payload)
			if _, err := conn.Write(buf.Bytes()); err != nil {
				log.Print(err)
				return
			}
		}(conn, channel, payload)
	}

	wg.Wait()
	log.Printf("[channel %s] end publish", channel)
}
