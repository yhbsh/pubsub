package client

import (
	"encoding/binary"
	"fmt"
	"net"
)

type Client struct {
	port int
}

func New(port int) *Client {
	return &Client{port: port}
}

func (client *Client) Publish(channel []byte, payload []byte) error {
	conn, err := net.Dial("unix", fmt.Sprintf("/tmp/.s.PUBSUB.%d", client.port))
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte{0}); err != nil {
		return err
	}

	if err := binary.Write(conn, binary.LittleEndian, uint8(len(channel))); err != nil {
		return err
	}

	if _, err := conn.Write(channel); err != nil {
		return err
	}

	if err := binary.Write(conn, binary.LittleEndian, uint32(len(payload))); err != nil {
		return err
	}

	if _, err := conn.Write(payload); err != nil {
		return err
	}

	return nil
}
