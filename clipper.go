// Copyright 2013 Wincent Colaiuta. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
//    this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
//    this list of conditions and the following disclaimer in the documentation
//    and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

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

// flags
var listenAddr string
var listenPort int

func init() {
	const (
		defaultListenAddr = "127.0.0.1"
		listenAddrUsage   = "address to bind to"
		defaultListenPort = 8377
		listenPortUsage   = "port to listen on"
		shorthand         = " (shorthand)"
	)

	flag.StringVar(&listenAddr, "address", defaultListenAddr, listenAddrUsage)
	flag.StringVar(&listenAddr, "a", defaultListenAddr, listenAddrUsage+shorthand)
	flag.IntVar(&listenPort, "port", defaultListenPort, listenPortUsage)
	flag.IntVar(&listenPort, "p", defaultListenPort, listenPortUsage+shorthand)
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
