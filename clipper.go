package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
)

const (
	PBCOPY = "pbcopy"
)

var listenAddr string
var listenPort int

func init() {
	const (
		defaultListenAddr = "127.0.0.1"
		listenAddrUsage = "address to bind to"
		defaultListenPort = 8377
		listenPortUsage = "port to listen on"
		shorthand = " (shorthand)"
	)

	flag.StringVar(&listenAddr, "address", defaultListenAddr, listenAddrUsage)
	flag.StringVar(&listenAddr, "a", defaultListenAddr, listenAddrUsage + shorthand)
	flag.IntVar(&listenPort, "port", defaultListenPort, listenPortUsage)
	flag.IntVar(&listenPort, "p", defaultListenPort, listenPortUsage + shorthand)
}

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		// additional command-line options not supported
		flag.Usage()
		os.Exit(1)
	}

	if _, err := exec.LookPath(PBCOPY); err != nil {
		log.Fatal(err.Error())
	}

	log.Print("Starting the server")
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", listenAddr, listenPort))
	if err != nil {
		log.Fatal(err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err.Error())
			return
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer log.Print("Connection closed")
	defer conn.Close()

	cmd := exec.Command(PBCOPY)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Print(err.Error())
		return
	}
	if err = cmd.Start(); err != nil {
		log.Print(err.Error())
		return
	}

	if copied, err := io.Copy(stdin, conn); err != nil {
		log.Print(err.Error())
	} else {
		log.Print("Echoed ", copied, " bytes")
	}
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		log.Print(err.Error())
	}
}
