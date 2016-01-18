package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/cheggaaa/pb"
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
	if fileInfo.IsDir() {
		fmt.Printf("Error, %s is dir!\n", os.Args[2])
		return
	}
	fileSize := fileInfo.Size()
	if fileSize <= 0 {
		fmt.Println("Empty file")
		return
	}
	titleid, serial := ciaTitleSerial(os.Args[2])
	fmt.Println("Installing cia file:", os.Args[2])
	fmt.Printf("Titleid: %016x, Seiral: %s\n", titleid, serial)

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

	//progress bar
	bar := pb.New64(fileSize)
	bar.SetRefreshRate(time.Second)
	bar.ShowCounters = false
	bar.ShowTimeLeft = true
	bar.ShowSpeed = true
	bar.SetUnits(pb.U_BYTES)
	pbStartFlag := false
	ciaInstallSuccess := false

	for {
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				bar.Set64(fileSize)
				ciaInstallSuccess = true
			} else {
				fmt.Println(err)
			}
			break
		}
		if n <= 0 {
			break
		}
		nWrite, err := out.Write(buffer[:n])
		if err != nil {
			fmt.Println(err)
			break
		}
		if pbStartFlag == false {
			pbStartFlag = true
			bar.Set64(0)
			bar.Start()
		}
		bar.Add(nWrite)
		if n != nWrite {
			fmt.Println("partial write...")
			break
		}
	}

	if ciaInstallSuccess {
		bar.FinishPrint("Install cia file success")
	} else {
		bar.FinishPrint("Install cia file failed")
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
	if (ext[0] == '.') &&
		(ext[1] == 'c' || ext[1] == 'C') &&
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
