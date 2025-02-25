package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func ReadU32(r io.Reader) ([]byte, error) {
	var chunkLen uint32
	if err := binary.Read(r, binary.LittleEndian, &chunkLen); err != nil {
		return nil, err
	}

	chunk := make([]byte, chunkLen)
	if n, err := io.ReadFull(r, chunk); err != nil {
		return nil, err
	} else if n != int(chunkLen) {
		return nil, errors.New("invalid chunk length")
	}

	return chunk, nil
}

func ReadU8(r io.Reader) ([]byte, error) {
	var chunkLen uint8
	if err := binary.Read(r, binary.LittleEndian, &chunkLen); err != nil {
		return nil, err
	}

	chunk := make([]byte, chunkLen)
	if n, err := io.ReadFull(r, chunk); err != nil {
		return nil, err
	} else if n != int(chunkLen) {
		return nil, errors.New("invalid chunk length")
	}

	return chunk, nil
}

func FmtSize(size int) string {
	const (
		KB    = 1024
		MB    = KB * 1024
		GB    = MB * 1024
		red   = "\033[31m" // Largest (GB, MB)
		green = "\033[32m" // Good (KB)
		blue  = "\033[34m" // Thin (bytes)
		reset = "\033[0m"  // Reset to default
	)

	if size >= GB {
		return fmt.Sprintf("%s%06.2f GB%s", red, float64(size)/float64(GB), reset)
	} else if size >= MB {
		return fmt.Sprintf("%s%06.2f MB%s", red, float64(size)/float64(MB), reset)
	} else if size >= KB {
		return fmt.Sprintf("%s%06.2f KB%s", green, float64(size)/float64(KB), reset)
	} else {
		return fmt.Sprintf("%s%03d bytes%s", blue, size, reset)
	}
}
