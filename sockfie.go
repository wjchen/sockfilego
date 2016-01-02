package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
)

const defaultPort = "5000"

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s <ip> <file>\n", os.Args[0])
		return
	}
	if isCiaFile(os.Args[2]) == false {
		fmt.Println("Error, not cia file")
		return
	}
	file, err := os.Open(os.Args[2])
	checkError(err)
	defer file.Close()
	fileInfo, err := file.Stat()
	checkError(err)
	fileSize := fileInfo.Size()
	ipPort := setDefaultPort(os.Args[1])
	raddr, err := net.ResolveTCPAddr("tcp", ipPort)
	checkError(err)
	out, err := net.DialTCP("tcp", nil, raddr)
	checkError(err)
	defer out.Close()
	buffer := make([]byte, 128*1024)

	//write filesize
	err = binary.Write(out, binary.BigEndian, &fileSize)
	checkError(err)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			fmt.Println(err)
			break
		}
		nWrite, err := out.Write(buffer[:n])
		if err != nil {
			fmt.Println(err)
			break
		}
		if n != nWrite {
			fmt.Println("partial write...")
			break
		}
	}
}

func setDefaultPort(ip string) string {
	_, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip + ":" + defaultPort
	}
	return ip
}

func isCiaFile(path string) bool {
	if len(path) < 4 {
		return false
	}
	ext := path[len(path)-4:]
	if len(ext) != 4 { //.cia
		return false
	}
	if (ext[1] == 'c' || ext[1] == 'C') &&
		(ext[2] == 'i' || ext[2] == 'i') &&
		(ext[3] == 'a' || ext[3] == 'A') {
		return true
	}
	return false
}

func checkError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
