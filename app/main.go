package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:2053")
	if err != nil {
		fmt.Println("Failed to resolve UDP address:", err)
		return
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Failed to bind to address:", err)
		return
	}
	defer udpConn.Close()

	buf := make([]byte, 512)

	for {
		size, source, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error receiving data:", err)
			break
		}

		receivedData := string(buf[:size])
		fmt.Printf("Received %d bytes from %s: %s\n", size, source, receivedData)

		dnsMessage := DNSMessage{}
		response := dnsMessage.writeResponse()

		_, err = udpConn.WriteToUDP(response, source)
		if err != nil {
			fmt.Println("Failed to send response:", err)
		}
	}
}

type DNSMessage struct {
	header   []byte
	question []byte
	answer   []byte
}

func (dm *DNSMessage) writeResponse() []byte {
	dm.writeHeader()
	dm.writeQuestion()
	res := make([]byte, 0, 32)
	res = append(res, dm.header...)
	res = append(res, dm.question...)
	return res
}

func (dm *DNSMessage) writeHeader() {
	header := make([]byte, 12)
	id := uint16(1234)
	binary.BigEndian.PutUint16(header[0:2], id)
	flags := buildFlags()
	binary.BigEndian.PutUint16(header[2:4], flags)
	qCount := uint16(0)
	binary.BigEndian.PutUint16(header[4:6], qCount)
	anCount := uint16(0)
	binary.BigEndian.PutUint16(header[6:8], anCount)
	nsCount := uint16(0)
	binary.BigEndian.PutUint16(header[8:10], nsCount)
	arCount := uint16(0)
	binary.BigEndian.PutUint16(header[10:12], arCount)
	dm.header = append(dm.header, header...)
}

func buildFlags() uint16 {
	flags := uint16(0)
	qr := uint16(1)
	flags |= qr << 15
	opcode := uint16(0)
	flags |= opcode << 11
	aa := uint16(0)
	flags |= aa << 10
	truncation := uint16(0)
	flags |= truncation << 9
	recursionDesired := uint16(0)
	flags |= recursionDesired << 8
	recursionAvailable := uint16(0)
	flags |= recursionAvailable << 7
	// 3 bits reserved
	rCode := uint16(0)
	flags |= rCode

	return flags
}

func (dm *DNSMessage) writeQuestion() {
	buf := bytes.NewBuffer([]byte{})

	qname, err := buildQuestionName("codecrafter.io")
	if err != nil {
		fmt.Println("Failed to build question name:", err)
	}
	buf.Write(qname)

	recordType := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, recordType); err != nil {
		fmt.Println("Failed to write recordType:", err)
		os.Exit(1)
	}

	recordClass := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, recordClass); err != nil {
		fmt.Println("Failed to write recordClass:", err)
		os.Exit(1)
	}
	dm.question = buf.Bytes()
}

func buildQuestionName(d string) ([]byte, error) {
	b := bytes.NewBuffer([]byte{})
	labelsArray := strings.Split(d, ".")
	for _, label := range labelsArray {
		l := len(label)
		if l > 63 {
			return nil, fmt.Errorf("label too long")
		}
		b.WriteByte(byte(l))
		b.WriteString(label)
	}
	b.WriteByte(0)
	return b.Bytes(), nil
}
