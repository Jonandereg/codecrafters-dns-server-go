package main

import (
	"bytes"
	"fmt"
	"net"
)

func (dm *DNSMessage) forwardRequest() ([]byte, error) {
	// Dial the resolver
	conn, err := net.Dial("udp", dm.forwardAddress)
	if err != nil {
		// handle error
	}
	defer conn.Close()
	baseHeader := dm.writeHeader(new(uint16(1)))
	buf := new(bytes.Buffer)
	for i := 0; i < int(dm.header.qCount); i++ {
		req, err := writeForwardRequest(baseHeader, dm.questions[i])
		if err != nil {
			return []byte{}, fmt.Errorf("writeForwardRequest error: %w", err)
		}

		_, err = conn.Write(req)
		if err != nil {
			return []byte{}, fmt.Errorf("error sending request: %w", err)
		}
		resp := make([]byte, 512)
		n, err := conn.Read(resp)

		if err != nil {
			return []byte{}, fmt.Errorf("error reading response: %w", err)
		}
		response := resp[:n]
		off := 12
		for {
			b := response[off]
			if b == 0x00 {
				off++
				break
			}
			if b&0xC0 == 0xC0 {
				off += 2
				break
			}
			off++
		}

		buf.Write(response[off+4:])
	}

	return buf.Bytes(), nil
}
func writeForwardRequest(header []byte, q Question) ([]byte, error) {
	buf := bytes.NewBuffer(header)
	questionBuf, err := writeQuestion(q)
	if err != nil {
		return nil, fmt.Errorf("")
	}
	buf.Write(questionBuf)
	return buf.Bytes(), nil
}
