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
	PBCOPY              = "pbcopy"
	DEFAULT_LISTEN_ADDR = "127.0.0.1"
	DEFAULT_LISTEN_PORT = 8377
)

var listenAddr string
var listenPort int

func init() {
	flag.StringVar(&listenAddr, "address", DEFAULT_LISTEN_ADDR, "address to bind to")
	flag.StringVar(&listenAddr, "a", DEFAULT_LISTEN_ADDR, "address to bind to (shorthand)")
	flag.IntVar(&listenPort, "port", DEFAULT_LISTEN_PORT, "port to listen on")
	flag.IntVar(&listenPort, "p", DEFAULT_LISTEN_PORT, "port to listen on (shorthand)")
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
