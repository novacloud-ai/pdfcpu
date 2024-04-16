package utils

import (
	"bufio"
	"bytes"
	"errors"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/log"
)

const (
	defaultBufSize = 1024
)

var ErrCorruptHeader = errors.New("pdfcpu: no header version available")

func growBufBy(buf []byte, size int, rd io.Reader) ([]byte, error) {
	b := make([]byte, size)

	if _, err := fillBuffer(rd, b); err != nil {
		return nil, err
	}
	//log.Read.Printf("growBufBy: Read %d bytes\n", n)

	return append(buf, b...), nil
}

func fillBuffer(r io.Reader, buf []byte) (int, error) {
	var n int
	var err error

	for n < len(buf) && err == nil {
		var nn int
		nn, err = r.Read(buf[n:])
		n += nn
	}

	if n > 0 && err == io.EOF {
		return n, nil
	}

	return n, err
}

func readStreamContentBlindly(rd io.Reader) (buf []byte, err error) {
	// Weak heuristic for reading in stream data for cases where stream length is unknown.
	// ...data...{eol}endstream{eol}endobj

	if buf, err = growBufBy(buf, defaultBufSize, rd); err != nil {
		return nil, err
	}

	i := bytes.Index(buf, []byte("endstream"))
	if i < 0 {
		for i = -1; i < 0; i = bytes.Index(buf, []byte("endstream")) {
			buf, err = growBufBy(buf, defaultBufSize, rd)
			if err != nil {
				return nil, err
			}
		}
	}

	buf = buf[:i]

	j := 0

	// Cut off trailing eol's.
	for i = len(buf) - 1; i >= 0 && (buf[i] == 0x0A || buf[i] == 0x0D); i-- {
		j++
	}

	if j > 0 {
		buf = buf[:len(buf)-j]
	}

	return buf, nil
}

func NewPositionedReader(rs io.ReadSeeker, offset *int64) (*bufio.Reader, error) {
	if _, err := rs.Seek(*offset, io.SeekStart); err != nil {
		return nil, err
	}

	if log.ReadEnabled() {
		log.Read.Printf("newPositionedReader: positioned to offset: %d\n", *offset)
	}

	return bufio.NewReader(rs), nil
}

// Reads and returns a file buffer with length = stream length using provided reader positioned at offset.
func ReadStreamContent(rd io.Reader, streamLength int) ([]byte, error) {
	if log.ReadEnabled() {
		log.Read.Printf("readStreamContent: begin streamLength:%d\n", streamLength)
	}

	if streamLength == 0 {
		// Read until "endstream" then fix "Length".
		return readStreamContentBlindly(rd)
	}

	buf := make([]byte, streamLength)

	for totalCount := 0; totalCount < streamLength; {
		count, err := fillBuffer(rd, buf[totalCount:])
		if err != nil {
			if err != io.EOF {
				return nil, err
			}
			// Weak heuristic to detect the actual end of this stream
			// once we have reached EOF due to incorrect streamLength.
			eob := bytes.Index(buf, []byte("endstream"))
			if eob < 0 {
				return nil, err
			}
			return buf[:eob], nil
		}

		if log.ReadEnabled() {
			log.Read.Printf("readStreamContent: count=%d, buflen=%d(%X)\n", count, len(buf), len(buf))
		}
		totalCount += count
	}

	if log.ReadEnabled() {
		log.Read.Printf("readStreamContent: end\n")
	}

	return buf, nil
}
