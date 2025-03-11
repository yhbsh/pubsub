package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	ADDR  = "0.0.0.0:9000"
	KB    = 1024
	MB    = KB * 1024
	GB    = MB * 1024
	RED   = "\033[31m"
	GREEN = "\033[32m"
	BLUE  = "\033[34m"
	RESET = "\033[0m"
)

var (
	channels = make(map[string]map[net.Conn]struct{})
	mutex    = sync.RWMutex{}
)

func rl32(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	payload := make([]byte, length)
	n, err := io.ReadFull(r, payload)
	if err != nil {
		return nil, err
	}

	if n != int(length) {
		return nil, errors.New("invalid chunk length")
	}

	return payload, nil
}

func rl8(r io.Reader) ([]byte, error) {
	var length uint8
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, err
	}

	payload := make([]byte, length)
	n, err := io.ReadFull(r, payload)
	if err != nil {
		return nil, err
	}

	if n != int(length) {
		return nil, errors.New("invalid chunk length")
	}

	return payload, nil
}

func FmtSize(size int) string {
	if size >= GB {
		return fmt.Sprintf("%s%06.2f GB%s", RED, float64(size)/float64(GB), RESET)
	} else if size >= MB {
		return fmt.Sprintf("%s%06.2f MB%s", RED, float64(size)/float64(MB), RESET)
	} else if size >= KB {
		return fmt.Sprintf("%s%06.2f KB%s", GREEN, float64(size)/float64(KB), RESET)
	} else {
		return fmt.Sprintf("%s%03d bytes%s", BLUE, size, RESET)
	}
}

func FmtDuration(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%s%03ds%s", RED, int64(d.Seconds()+0.5), RESET)
	} else if d >= time.Millisecond {
		return fmt.Sprintf("%s%03dms%s", GREEN, int64(float64(d.Microseconds())/1e3+0.5), RESET)
	} else {
		return fmt.Sprintf("%s%03dµs%s", BLUE, d.Microseconds(), RESET)
	}
}

func Subscribe(conn net.Conn, channel string) {
	mutex.Lock()
	defer mutex.Unlock()

	if _, ok := channels[channel]; !ok {
		channels[channel] = make(map[net.Conn]struct{})
	}

	channels[channel][conn] = struct{}{}
	log.Printf("[conn %s] [subscribed %s]", conn.RemoteAddr(), string(channel))
}

func Unsubscribe(conn net.Conn, channel string) {
	mutex.Lock()
	defer mutex.Unlock()

	if clients, ok := channels[channel]; ok {
		delete(clients, conn)
		if len(clients) == 0 {
			delete(channels, channel)
		}
		log.Printf("[conn %s] [unsubscribed %s]", conn.RemoteAddr(), string(channel))
	}
}

func Publish(channel []byte, payload []byte) {
	mutex.Lock()
	conns, exists := channels[string(channel)]
	mutex.Unlock()

	log.Printf("[channel %s] begin publish", channel)
	if !exists {
		log.Printf("[channel %s] end publish, no clients", channel)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(conns))

	for conn := range conns {
		go func(conn net.Conn, channel []byte, payload []byte) {
			defer wg.Done()
			startTime := time.Now()

			log.Printf("[channel %s] begin publish to client %s", channel, conn.RemoteAddr())

			var buf bytes.Buffer
			buf.WriteByte(byte(len(channel)))
			buf.Write(channel)
			buf.WriteByte(byte(len(payload) >> (8 * 0)))
			buf.WriteByte(byte(len(payload) >> (8 * 1)))
			buf.WriteByte(byte(len(payload) >> (8 * 2)))
			buf.WriteByte(byte(len(payload) >> (8 * 3)))
			buf.Write(payload)

			if _, err := conn.Write(buf.Bytes()); err != nil {
				log.Print(err)
				return
			}

			elapsed := time.Since(startTime)
			log.Printf("[channel %s] end publish to client %s [time %s]", channel, conn.RemoteAddr(), FmtDuration(elapsed))
		}(conn, channel, payload)
	}

	wg.Wait()
	log.Printf("[channel %s] end publish", channel)
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()

		addr, err := net.ResolveTCPAddr("tcp4", ADDR)
		if err != nil {
			log.Fatal(err)
		}
		ln, err := net.ListenTCP("tcp4", addr)
		if err != nil {
			log.Fatal(err)
		}
		defer ln.Close()

		log.Printf("Listening on %s", ADDR)
		for {
			conn, err := ln.AcceptTCP()
			if err != nil {
				log.Printf("Accept error %v", err)
				continue
			}

			go HandleSub(conn)
		}
	}()

	go func() {
		defer wg.Done()

		unixSocketPath := "/tmp/pubsub.sock"
		if err := os.RemoveAll(unixSocketPath); err != nil {
			log.Fatal(err)
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

func HandleSub(conn *net.TCPConn) {
	log.Printf("[conn %s] connected", conn.RemoteAddr())

	defer func() {
		log.Printf("[conn %s] disconnected", conn.RemoteAddr())
		conn.Close()

		mutex.Lock()
		defer mutex.Unlock()
		for channel := range channels {
			delete(channels[channel], conn)
		}
	}()

	for {
		var commandType uint8
		if err := binary.Read(conn, binary.LittleEndian, &commandType); err != nil {
			log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
			return
		}

		channel, err := rl8(conn)
		if err != nil {
			log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
			return
		}

		switch commandType {
		case 1:
			Subscribe(conn, string(channel))
		case 2:
			Unsubscribe(conn, string(channel))
		default:
			log.Printf("[conn %s] invalid command type", conn.RemoteAddr())
		}
	}
}

func HandlePub(conn *net.UnixConn) {
	defer conn.Close()

	var cmdType uint8
	if err := binary.Read(conn, binary.LittleEndian, &cmdType); err != nil {
		log.Printf("[conn %s] %v", conn.RemoteAddr(), err)
		return
	}

	if cmdType != 0 {
		log.Printf("[IP unix] invalid command from unix domain socket, valid command is publish only")
		return
	}

	channel, err := rl8(conn)
	if err != nil {
		log.Printf("[IP unix] %v", err)
		return
	}

	payload, err := rl32(conn)
	if err != nil {
		log.Printf("[IP unix] %v", err)
		return
	}

	go Publish(channel, payload)
}
