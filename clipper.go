// Copyright 2013-present Greg Hurrell. All rights reserved.
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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"syscall"
	"strings"
)

const (
	pbcopy            = "pbcopy"
	defaultListenAddr = "127.0.0.1"
	defaultListenPort = 8377
	defaultLogFile    = "~/Library/Logs/com.wincent.clipper.log"
	defaultConfigFile = "~/.clipper.json"
)

type Config struct {
	Address string
}

var config Config

// flags
var listenAddr, logFile, configFile string
var listenPort int

func init() {
	const (
		listenAddrUsage = "address to bind to"
		listenPortUsage = "port to listen on"
		logFileUsage    = "path to logfile"
		configFileUsage = "path to (JSON) config file"
		shorthand       = " (shorthand)"
	)

	flag.StringVar(&listenAddr, "address", defaultListenAddr, listenAddrUsage)
	flag.StringVar(&listenAddr, "a", defaultListenAddr, listenAddrUsage+shorthand)
	flag.IntVar(&listenPort, "port", defaultListenPort, listenPortUsage)
	flag.IntVar(&listenPort, "p", defaultListenPort, listenPortUsage+shorthand)
	flag.StringVar(&logFile, "logfile", defaultLogFile, logFileUsage)
	flag.StringVar(&logFile, "l", defaultLogFile, logFileUsage)
	flag.StringVar(&configFile, "config", defaultConfigFile, configFileUsage)
	flag.StringVar(&configFile, "c", defaultConfigFile, configFileUsage+shorthand)
}

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		// additional command-line options not supported
		flag.Usage()
		os.Exit(1)
	}

	expandedPath := pathByExpandingTildeInPath(logFile)
	outfile, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()
	log.SetOutput(outfile)
	log.SetPrefix("clipper: ")

	if _, err := exec.LookPath(pbcopy); err != nil {
		log.Fatal(err)
	}

	expandedPath = pathByExpandingTildeInPath(configFile)
	if configData, err := ioutil.ReadFile(expandedPath); err != nil {
		if configFile == defaultConfigFile {
			// default config file missing; just warn
			log.Print(err)
		} else {
			// user explicitly asked for non-default config file; fail hard
			log.Fatal(err)
		}
	} else {
		if err = json.Unmarshal(configData, &config); err != nil {
			log.Fatal(err)
		}
	}

	var addr string
	var listenType string
	if (config.Address != "") {
		addr = pathByExpandingTildeInPath(config.Address)
	} else {
		addr = pathByExpandingTildeInPath(listenAddr)
	}
	if strings.HasPrefix(addr, "/") {
		log.Print("Starting UNIX domain socket server at ", addr)
		listenType = "unix"
	} else {
		log.Print("Starting TCP server on ", addr)
		listenType = "tcp"
		addr = fmt.Sprintf("%s:%d", listenAddr, listenPort)
	}
	listener, err := net.Listen(listenType, addr)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Print(err)
				return
			}

			go handleConnection(conn)
		}
	}()

	// Need to catch signals in order for `defer`-ed clean-up items to run.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)
	sig := <-c
	log.Print("Got signal ", sig)
}

func pathByExpandingTildeInPath(path string) string {
	if strings.HasPrefix(path, "~") {
		user, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		path = user.HomeDir + path[1:]
	}
	return path
}

func handleConnection(conn net.Conn) {
	defer log.Print("Connection closed")
	defer conn.Close()

	cmd := exec.Command(pbcopy)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Print(err)
		return
	}
	if err = cmd.Start(); err != nil {
		log.Print(err)
		return
	}

	if copied, err := io.Copy(stdin, conn); err != nil {
		log.Print(err)
	} else {
		log.Print("Echoed ", copied, " bytes")
	}
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		log.Print(err)
	}
}
