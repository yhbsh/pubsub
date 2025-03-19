package utils

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	KB    = 1024
	MB    = KB * 1024
	GB    = MB * 1024
	RED   = "\033[31m"
	GREEN = "\033[32m"
	BLUE  = "\033[34m"
	RESET = "\033[0m"
)

func R32le(r io.Reader) ([]byte, error) {
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

func R8le(r io.Reader) ([]byte, error) {
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

func FmtTime(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%s%03ds%s", RED, int64(d.Seconds()+0.5), RESET)
	} else if d >= time.Millisecond {
		return fmt.Sprintf("%s%03dms%s", GREEN, int64(float64(d.Microseconds())/1e3+0.5), RESET)
	} else {
		return fmt.Sprintf("%s%03dµs%s", BLUE, d.Microseconds(), RESET)
	}
}
