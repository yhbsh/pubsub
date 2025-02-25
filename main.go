package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"log"
	"net"
	"os"
	"slices"
	"sync"
)

const (
	CmdTypePublish     byte = 0
	CmdTypeSubscribe   byte = 1
	CmdTypeUnsubscribe byte = 2
)

var (
	subscriptions = make(map[net.Conn][]string)
	mu            = sync.RWMutex{}
)

func Publish(conn net.Conn, channel []byte, payload []byte) {
	var buf bytes.Buffer

	buf.WriteByte(uint8(len(channel)))
	buf.Write(channel)
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(payload))); err != nil {
		log.Printf("failed to write payload length: %v", err)
		return
	}
	buf.Write(payload)

	if _, err := conn.Write(buf.Bytes()); err != nil {
		mu.Lock()
		delete(subscriptions, conn)
		mu.Unlock()
		log.Print(err)
		return
	}

	log.Printf("[IP %s] [publish %s] [payload %s]", conn.RemoteAddr(), channel, FmtSize(len(payload)))

}

func isSubscribed(channels []string, target string) bool {
	for i := range channels {
		ch := channels[i]
		if ch == target {
			return true
		}
	}
	return false
}

func removeChannel(channels []string, target string) []string {
	for i, ch := range channels {
		if ch == target {
			return slices.Delete(channels, i, i+1)
		}
	}
	return channels
}

func HandleSub(conn *net.TCPConn) {
	log.Printf("[IP %s] connected", conn.RemoteAddr())

	defer func() {
		log.Printf("[IP %s] disconnected", conn.RemoteAddr())
		conn.Close()

		mu.Lock()
		delete(subscriptions, conn)
		mu.Unlock()
	}()

	for {
		var cmdType uint8
		if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
			if err != io.EOF {
				log.Printf("[IP %s] %v", conn.RemoteAddr(), err)
			}
			return
		}

		if cmdType != CmdTypeSubscribe && cmdType != CmdTypeUnsubscribe {
			log.Printf("[IP %s] invalid command type", conn.RemoteAddr())
			return
		}

		if cmdType == CmdTypeSubscribe {
			channel, err := ReadU8(conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("[IP %s] %v", conn.RemoteAddr(), err)
				}
				return
			}

			mu.Lock()
			if subscriptions[conn] == nil {
				subscriptions[conn] = []string{}
			}

			if isSubscribed(subscriptions[conn], string(channel)) {
				continue
			}

			subscriptions[conn] = append(subscriptions[conn], string(channel))
			mu.Unlock()

			log.Printf("[IP %s] [subscribed %s]", conn.RemoteAddr(), string(channel))
		}

		if cmdType == CmdTypeUnsubscribe {
			channel, err := ReadU8(conn)
			if err != nil {
				if err != io.EOF {
					log.Printf("[IP %s] %v", conn.RemoteAddr(), err)
				}
				return
			}

			mu.Lock()
			if channels, exists := subscriptions[conn]; exists {
				subscriptions[conn] = removeChannel(channels, string(channel))

				if len(subscriptions[conn]) == 0 {
					delete(subscriptions, conn)
				}
			}
			mu.Unlock()

			log.Printf("[IP %s] [unsubscribed %s]", conn.RemoteAddr(), string(channel))
		}
	}
}

func HandlePub(conn *net.UnixConn) {
	defer func() {
		conn.Close()

		mu.Lock()
		delete(subscriptions, conn)
		mu.Unlock()
	}()

	for {
		var cmdType uint8
		if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
			if err != io.EOF {
				log.Printf("[IP %s] %v", conn.RemoteAddr(), err)
			}
			return
		}

		if cmdType != CmdTypePublish {
			log.Printf("[IP unix] invalid command from unix domain socket, valid command is publish only")
			return
		}

		channel, err := ReadU8(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("[IP unix] %v", err)
			}
			return
		}

		payload, err := ReadU32(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("[IP unix] %v", err)
			}
			return
		}

		go func(channel []byte, payload []byte) {
			log.Printf("[channel %s] publish .....", channel)
			mu.RLock()
			for conn, channels := range subscriptions {
				if !isSubscribed(channels, string(channel)) {
					continue
				}

				go Publish(conn, channel, payload)
			}
			mu.RUnlock()
		}(channel, payload)
	}
}
func main() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:51011")
		if err != nil {
			log.Fatal(err)
		}

		ln, err := net.ListenTCP("tcp", addr)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("Listening on 0.0.0.0:51011 (pubsub)")
		defer ln.Close()

		for {
			conn, err := ln.AcceptTCP()
			if err != nil {
				log.Print("Accept error:", err)
				continue
			}

			go HandleSub(conn)
		}
	}()

	go func() {
		defer wg.Done()

		unixSocketPath := "/tmp/pubsub.sock"
		if err := os.RemoveAll(unixSocketPath); err != nil {
			log.Print(err)
			return
		}

		addr, err := net.ResolveUnixAddr("unix", unixSocketPath)
		if err != nil {
			log.Fatal(err)
		}
		ln, err := net.ListenUnix("unix", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()

		for {
			conn, err := ln.AcceptUnix()
			if err != nil {
				log.Print("Accept error:", err)
				continue
			}

			go HandlePub(conn)
		}
	}()

	wg.Wait()
}
