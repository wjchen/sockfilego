package main

import (
	"encoding/binary"
	"errors"
	"os"
)

type ciaFileHeader struct {
	HeaderSize   uint32
	FileType     uint16
	Version      uint16
	ChainSize    uint32
	TicketSize   uint32
	TmdSize      uint32
	MetaSize     uint32
	ContentSize  uint64
	ContentIndex [0x2000]uint8
}

//sigtype
type sigHeader struct {
	sigId      uint32
	sigStr     string
	sigSize    uint32
	sigPadSize uint32
}

const (
	RSA4096_SHA256      = 0x10003
	RSA4096_SHA256_STR  = "RSA4096_SHA256"
	RSA4096_SHA256_SIZE = 0x200
	RSA4096_SHA256_PAD  = 0x3C

	RSA2048_SHA256      = 0x10004
	RSA2048_SHA256_STR  = "RSA2048_SHA256"
	RSA2048_SHA256_SIZE = 0x100
	RSA2048_SHA256_PAD  = 0x3C

	ECDSA_SHA256      = 0x10005
	ECDSA_SHA256_STR  = "ECDSA_SHA256"
	ECDSA_SHA256_SIZE = 0x3C
	ECDSA_SHA256_PAD  = 0x40
)

var availSigMap = map[uint32]sigHeader{
	RSA4096_SHA256: sigHeader{
		sigId:      RSA4096_SHA256,
		sigStr:     RSA4096_SHA256_STR,
		sigSize:    RSA4096_SHA256_SIZE,
		sigPadSize: RSA4096_SHA256_PAD,
	},
	RSA2048_SHA256: sigHeader{
		sigId:      RSA2048_SHA256,
		sigStr:     RSA2048_SHA256_STR,
		sigSize:    RSA2048_SHA256_SIZE,
		sigPadSize: RSA2048_SHA256_PAD,
	},
	ECDSA_SHA256: sigHeader{
		sigId:      ECDSA_SHA256,
		sigStr:     ECDSA_SHA256_STR,
		sigSize:    ECDSA_SHA256_SIZE,
		sigPadSize: ECDSA_SHA256_PAD,
	},
}

type ticketData struct {
	Issuer             [0x40]byte
	ECCKey             [0x3c]byte
	Version            byte
	CaCrlVersion       byte
	SignerCrlVersion   byte
	TitleKey           [0x10]byte
	Reserved           byte
	TitleID            uint64
	ConsoleID          uint32
	TitleID2           uint64
	Reserved2          uint16
	TicketTitleVersion uint16
	Reserved3          uint64
	LicenseType        byte
	KeyIndex           byte
	Reserved4          [0x2a]byte
	EShopID            uint32
	Reserved5          byte
	Audit              byte
	Reserved6          [0x42]byte
	Limits             [0x40]byte
	ContentIndex       [0xac]byte
}

func align64(size uint64) uint64 {
	remain := size & 63
	if remain == 0 {
		return size
	}
	return size + (64 - remain)
}

func readTicket(f *os.File, offset, ticketLen uint64) (uint64, error) {
	_, err := f.Seek(int64(offset), os.SEEK_SET)
	if err != nil {
		return 0, err
	}
	var sigType uint32
	err = binary.Read(f, binary.BigEndian, &sigType)
	if err != nil {
		return 0, err
	}
	sig, ok := availSigMap[sigType]
	if !ok {
		return 0, errors.New("Unkown sig type")
	}
	tickDataOffset := align64(uint64(sig.sigSize + sig.sigPadSize))
	_, err = f.Seek(int64(offset+tickDataOffset), os.SEEK_SET)
	if err != nil {
		return 0, err
	}
	tikData := ticketData{}
	err = binary.Read(f, binary.BigEndian, &tikData)
	if err != nil {
		return 0, err
	}
	return tikData.TitleID2, nil
}

const serialOffset = 336 //not sure

func readContentSerial(f *os.File, offset, ticketLen uint64) string {
	contentHeadRead := 256
	_, err := f.Seek(int64(offset+serialOffset), os.SEEK_SET)
	if err != nil {
		return ""
	}
	contentBuf := make([]byte, contentHeadRead)
	_, err = f.Read(contentBuf)
	if err != nil {
		return ""
	}
	serialEnd := 0
	for i := 0; i < contentHeadRead; i++ {
		if contentBuf[i] == 0 {
			serialEnd = i
			break
		}
	}
	return string(contentBuf[:serialEnd])
}

func ciaTitleSerial(path string) (uint64, string) {
	ciaFile, err := os.Open(path)
	if err != nil {
		return 0, ""
	}
	defer ciaFile.Close()
	h := ciaFileHeader{}
	err = binary.Read(ciaFile, binary.LittleEndian, &h)
	if err != nil {
		return 0, ""
	}
	certOffset := align64(uint64(h.HeaderSize))
	ticketOffset := align64(certOffset + uint64(h.ChainSize))

	tmdOffset := align64(ticketOffset + uint64(h.TicketSize))

	contentOffset := align64(tmdOffset + uint64(h.TmdSize))
	metaOffset := align64(contentOffset + uint64(h.ContentSize))
	ciaFileInfo, err := ciaFile.Stat()
	if err != nil {
		return 0, ""
	}
	if uint64(ciaFileInfo.Size()) != align64((metaOffset + uint64(h.MetaSize))) {
		//size may not right, not sure
	}
	titleid, err := readTicket(ciaFile, ticketOffset, uint64(h.TicketSize))
	if err != nil {
		return 0, ""
	}
	serial := readContentSerial(ciaFile, contentOffset, uint64(h.ContentSize))
	return titleid, serial
}
