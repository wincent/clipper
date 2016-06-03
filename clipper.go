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
	"path/filepath"
	"strings"
	"syscall"
)

type Options struct {
	Address string
	Config  string
	Logfile string
	Port    int
}

var config Options // Options read from disk.
var flags Options  // Options set via commandline flags.
var defaults = Options{
	Address: "127.0.0.1",
	Config:  "~/.clipper.json",
	Logfile: "~/Library/Logs/com.wincent.clipper.log",
	Port:    8377,
}

const (
	pbcopy = "pbcopy"
)

func init() {
	const (
		listenAddrUsage = "address to bind to"
		listenPortUsage = "port to listen on"
		logFileUsage    = "path to logfile"
		configFileUsage = "path to (JSON) config file"
		shorthand       = " (shorthand)"
	)

	flag.StringVar(&flags.Address, "address", defaults.Address, listenAddrUsage)
	flag.StringVar(&flags.Address, "a", defaults.Address, listenAddrUsage+shorthand)
	flag.IntVar(&flags.Port, "port", defaults.Port, listenPortUsage)
	flag.IntVar(&flags.Port, "p", defaults.Port, listenPortUsage+shorthand)
	flag.StringVar(&flags.Logfile, "logfile", defaults.Logfile, logFileUsage)
	flag.StringVar(&flags.Logfile, "l", defaults.Logfile, logFileUsage)
	flag.StringVar(&flags.Config, "config", defaults.Config, configFileUsage)
	flag.StringVar(&flags.Config, "c", defaults.Config, configFileUsage+shorthand)
}

func main() {
	flag.Parse()
	if flag.NArg() != 0 {
		// additional command-line options not supported
		flag.Usage()
		os.Exit(1)
	}

	expandedPath := expandPath(flags.Logfile)
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

	expandedPath = expandPath(flags.Config)
	if configData, err := ioutil.ReadFile(expandedPath); err != nil {
		if flags.Config == defaults.Config {
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
	if isPath(config.Address) {
		addr = expandPath(config.Address)
	} else if isPath(flags.Address) {
		addr = expandPath(flags.Address)
	} else {
		addr = flags.Address
	}
	if strings.HasPrefix(addr, "/") {
		log.Print("Starting UNIX domain socket server at ", addr)
		listenType = "unix"
	} else {
		log.Print("Starting TCP server on ", addr)
		listenType = "tcp"
		addr = fmt.Sprintf("%s:%d", flags.Address, flags.Port)
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

// Returns true for things which look like paths (start with "~", "." or "/").
func isPath(path string) bool {
	return strings.HasPrefix(path, "~") ||
		strings.HasPrefix(path, ".") ||
		strings.HasPrefix(path, "/")
}

func expandPath(path string) string {
	expanded := pathByExpandingTildeInPath(path)
	result, err := filepath.Abs(expanded)
	if err != nil {
		log.Fatal(err)
	}
	return result
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
