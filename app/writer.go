package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
)

func (dm *DNSMessage) writeResponse() ([]byte, error) {
	q, err := dm.writeQuestions()
	if err != nil {
		return nil, fmt.Errorf("failed to write questions: %v", err)
	}

	a, err := dm.writeAnswer()
	if err != nil {
		return nil, fmt.Errorf("failed to write answer: %v", err)
	}

	h := dm.writeHeader(nil)
	res := make([]byte, 0, 32)
	res = append(res, h...)
	res = append(res, q...)
	res = append(res, a...)
	return res, nil
}

func (dm *DNSMessage) writeHeader(qCount *uint16) []byte {
	header := make([]byte, 12)

	binary.BigEndian.PutUint16(header[0:2], dm.header.ID)
	// if qCount is not nil this is a forwarding request
	flags := writeFlags(dm.header.Flags, qCount != nil)

	binary.BigEndian.PutUint16(header[2:4], flags)
	if qCount != nil {
		binary.BigEndian.PutUint16(header[4:6], *qCount)
		binary.BigEndian.PutUint16(header[6:8], uint16(0))
	} else {
		binary.BigEndian.PutUint16(header[4:6], dm.header.qCount)
		binary.BigEndian.PutUint16(header[6:8], dm.header.anCount)
	}
	nsCount := uint16(0)
	binary.BigEndian.PutUint16(header[8:10], nsCount)
	arCount := uint16(0)
	binary.BigEndian.PutUint16(header[10:12], arCount)
	return header
}

func writeFlags(f Flags, isForwarding bool) uint16 {
	flags := uint16(0)
	var qr uint16
	if isForwarding {
		qr = uint16(0)
	} else {
		qr = uint16(1)
	}
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
	rCode := uint16(0)
	if opcode != 0 {
		rCode = uint16(4)
	}

	flags |= rCode

	return flags
}

func (dm *DNSMessage) writeQuestions() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	for i := 0; i < int(dm.header.qCount); i++ {
		question, err := writeQuestion(dm.questions[i])
		if err != nil {
			return nil, fmt.Errorf("failed to write question: %v", err)
		}
		buf.Write(question)
	}

	return buf.Bytes(), nil
}

func writeName(d string) ([]byte, error) {
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

func writeQuestion(q Question) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	qname, err := writeName(q.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to build questionMsg name: %v", err)
	}
	buf.Write(qname)

	recordType := q.Type
	if err := binary.Write(buf, binary.BigEndian, recordType); err != nil {
		return nil, fmt.Errorf("failed to write recordType: %v", err)
	}

	recordClass := q.Class
	if err := binary.Write(buf, binary.BigEndian, recordClass); err != nil {
		return nil, fmt.Errorf("failed to write recordClass: %v", err)
	}
	return buf.Bytes(), nil
}

func (dm *DNSMessage) writeAnswer() ([]byte, error) {
	dm.header.anCount = dm.header.qCount
	buf := bytes.NewBuffer([]byte{})
	if dm.forwardAddress == "" {
		testIp := net.ParseIP("127.0.0.1").To4()
		for i := 0; i < int(dm.header.qCount); i++ {
			question, err := writeQuestion(dm.questions[i])
			if err != nil {
				return nil, fmt.Errorf("failed to write question: %v", err)
			}
			buf.Write(question)
			ttl := uint32(60)
			if err := binary.Write(buf, binary.BigEndian, ttl); err != nil {
				return nil, fmt.Errorf("failed to write ttl: %v", err)
			}
			length := uint16(len(testIp))
			if err := binary.Write(buf, binary.BigEndian, length); err != nil {
				return nil, fmt.Errorf("failed to write length: %v", err)
			}
			buf.Write(testIp)
		}

		return buf.Bytes(), nil
	}

	answers, err := dm.forwardRequest()
	if err != nil {
		return nil, fmt.Errorf("failed to forward request: %v", err)
	}

	return answers, nil
}
