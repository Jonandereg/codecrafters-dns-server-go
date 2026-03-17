package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
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
		rawRequest := buf[:size]
		receivedData := string(rawRequest)
		fmt.Printf("Received %d bytes from %s: %s\n", size, source, receivedData)

		dnsMessage := DNSMessage{
			rawRequest: rawRequest,
		}
		if err := dnsMessage.parseQuery(); err != nil {
			fmt.Println("Failed to parse DNS message:", err)
		}
		response, err := dnsMessage.writeResponse()
		if err != nil {
			fmt.Println("Error writing response:", err)
			break
		}

		_, err = udpConn.WriteToUDP(response, source)
		if err != nil {
			fmt.Println("Failed to send response:", err)
		}
	}
}

type DNSMessage struct {
	rawRequest []byte
	header     Header
}
type Header struct {
	ID      uint16
	qCount  uint16
	anCount uint16
	Flags
}
type Flags struct {
	qr         uint16
	opCode     uint16
	rd         uint16
	aa         uint16
	truncation uint16
	rCode      uint16
}

func (dm *DNSMessage) writeResponse() ([]byte, error) {
	q, err := dm.writeQuestion()

	if err != nil {
		return nil, fmt.Errorf("failed to write question: %v", err)
	}
	a, err := dm.writeAnswer()
	if err != nil {
		return nil, fmt.Errorf("failed to write answer: %v", err)
	}
	h := dm.writeHeader()
	res := make([]byte, 0, 32)
	res = append(res, h...)
	res = append(res, q...)
	res = append(res, a...)
	return res, nil
}

func (dm *DNSMessage) writeHeader() []byte {
	header := make([]byte, 12)

	binary.BigEndian.PutUint16(header[0:2], dm.header.ID)

	flags := writeFlags(dm.header.Flags)

	binary.BigEndian.PutUint16(header[2:4], flags)
	binary.BigEndian.PutUint16(header[4:6], dm.header.qCount)
	binary.BigEndian.PutUint16(header[6:8], dm.header.anCount)
	nsCount := uint16(0)
	binary.BigEndian.PutUint16(header[8:10], nsCount)
	arCount := uint16(0)
	binary.BigEndian.PutUint16(header[10:12], arCount)
	return header
}

func writeFlags(f Flags) uint16 {
	flags := uint16(0)
	qr := uint16(1)
	flags |= qr << 15
	opcode := f.opCode
	flags |= opcode << 11
	aa := f.aa
	flags |= aa << 10
	truncation := f.truncation
	flags |= truncation << 9
	recursionDesired := f.rd
	flags |= recursionDesired << 8
	recursionAvailable := uint16(0)
	flags |= recursionAvailable << 7
	// 3 bits reserved
	rCode := f.rCode
	flags |= rCode

	return flags
}

func (dm *DNSMessage) writeQuestion() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})

	qname, err := buildQuestionName("codecrafters.io")
	if err != nil {
		return nil, fmt.Errorf("failed to build questionMsg name: %v", err)
	}
	buf.Write(qname)

	recordType := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, recordType); err != nil {
		return nil, fmt.Errorf("failed to write recordType: %v", err)
	}

	recordClass := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, recordClass); err != nil {
		return nil, fmt.Errorf("failed to write recordClass: %v", err)
	}
	dm.header.qCount = uint16(1)

	return buf.Bytes(), nil
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

func (dm *DNSMessage) writeAnswer() ([]byte, error) {
	testIp := net.ParseIP("127.0.0.1").To4()
	buf := bytes.NewBuffer([]byte{})
	name, err := buildQuestionName("codecrafters.io")
	if err != nil {
		return nil, fmt.Errorf("failed to build questionMsg name: %v", err)
	}
	buf.Write(name)
	recordType := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, recordType); err != nil {
		return nil, fmt.Errorf("failed to write recordType: %v", err)
	}
	classType := uint16(1)
	if err := binary.Write(buf, binary.BigEndian, classType); err != nil {
		return nil, fmt.Errorf("failed to write classType: %v", err)
	}
	ttl := uint32(60)
	if err := binary.Write(buf, binary.BigEndian, ttl); err != nil {
		return nil, fmt.Errorf("failed to write ttl: %v", err)
	}
	length := uint16(len(testIp))
	if err := binary.Write(buf, binary.BigEndian, length); err != nil {
		return nil, fmt.Errorf("failed to write length: %v", err)
	}
	buf.Write(testIp)
	dm.header.anCount = uint16(1)

	return buf.Bytes(), nil
}

func (dm *DNSMessage) parseQuery() error {
	if err := dm.parseHeader(); err != nil {
		return fmt.Errorf("failed to parse header: %v", err)
	}
	return nil
}

func (dm *DNSMessage) parseHeader() error {
	if len(dm.rawRequest) < 12 {
		return fmt.Errorf("bad request, header is too short")
	}
	rawHeader := dm.rawRequest[:12]
	id := binary.BigEndian.Uint16(rawHeader[0:2])
	flags := binary.BigEndian.Uint16(rawHeader[2:4])
	f := parseFlags(flags)
	dm.header.ID = id
	dm.header.Flags = f
	return nil
}

func parseFlags(f uint16) Flags {
	qr := (f >> 15) & 1
	opcode := (f >> 11) & 0b1111
	aa := (f >> 10) & 1
	truncation := (f >> 9) & 1
	rd := (f >> 8) & 1
	rCode := f & 0b1111
	return Flags{
		qr:         qr,
		opCode:     opcode,
		rd:         rd,
		aa:         aa,
		truncation: truncation,
		rCode:      rCode,
	}
}
