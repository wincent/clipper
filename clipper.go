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
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

type Options struct {
	Address    string
	Config     string
	Logfile    string
	Executable string
	Flags      string
	Port       int
}

var version = "unknown"

var config Options   // Options read from disk.
var defaults Options // Default options.
var flags Options    // Options set via commandline flags.
var settings Options // Result of merging: flags > config > defaults.
var showHelp bool
var showVersion bool

func printVersion() {
	fmt.Fprintf(os.Stderr, "clipper version: %s (%s)\n", version, runtime.GOOS)
}

func initFlags() {
	const (
		flagsUsage      = "arguments passed to clipboard executable"
		configFileUsage = "path to (JSON) config file"
		executableUsage = "program called to write to clipboard"
		helpUsage       = "show usage information"
		listenAddrUsage = "address to bind to (default loopback interface)"
		listenPortUsage = "port to listen on"
		logFileUsage    = "path to logfile"
		versionUsage    = "show version information"
	)

	flag.BoolVar(&showHelp, "h", false, helpUsage)
	flag.BoolVar(&showHelp, "help", false, helpUsage)
	flag.IntVar(&flags.Port, "p", defaults.Port, listenPortUsage)
	flag.IntVar(&flags.Port, "port", defaults.Port, listenPortUsage)
	flag.StringVar(&flags.Address, "a", defaults.Address, listenAddrUsage)
	flag.StringVar(&flags.Address, "address", defaults.Address, listenAddrUsage)
	flag.StringVar(&flags.Config, "c", defaults.Config, configFileUsage)
	flag.StringVar(&flags.Config, "config", defaults.Config, configFileUsage)
	flag.StringVar(&flags.Executable, "e", defaults.Executable, executableUsage)
	flag.StringVar(&flags.Executable, "executable", defaults.Executable, executableUsage)
	flag.StringVar(&flags.Flags, "f", defaults.Flags, flagsUsage)
	flag.StringVar(&flags.Flags, "flags", defaults.Flags, flagsUsage)
	flag.StringVar(&flags.Logfile, "l", defaults.Logfile, logFileUsage)
	flag.StringVar(&flags.Logfile, "logfile", defaults.Logfile, logFileUsage)
	flag.BoolVar(&showVersion, "v", false, versionUsage)
	flag.BoolVar(&showVersion, "version", false, versionUsage)
}

func setDefaults() {
	defaults.Address = "" // IPv4/IPv6 loopback.
	defaults.Port = 8377

	if runtime.GOOS == "linux" {
		defaults.Config = "~/.config/clipper/clipper.json"
		defaults.Logfile = "~/.config/clipper/logs/clipper.log"
		defaults.Executable = "xclip"
		defaults.Flags = "-selection clipboard"
	} else {
		defaults.Config = "~/.clipper.json"
		defaults.Logfile = "~/Library/Logs/com.wincent.clipper.log"
		defaults.Executable = "pbcopy"
		defaults.Flags = ""
	}
}

func mergeSettings() {
	// Detect which flags were passed in explicitly, and set them immediately.
	// This is used below to determine response to a missing config file.
	visitor := func(f *flag.Flag) {
		if f.Name == "address" || f.Name == "a" {
			settings.Address = flags.Address
		} else if f.Name == "config" || f.Name == "c" {
			settings.Config = flags.Config
		} else if f.Name == "executable" || f.Name == "e" {
			settings.Executable = flags.Executable
		} else if f.Name == "flags" || f.Name == "f" {
			settings.Flags = flags.Flags
		} else if f.Name == "port" || f.Name == "p" {
			settings.Port = flags.Port
		} else if f.Name == "logfile" || f.Name == "l" {
			settings.Logfile = flags.Logfile
		}
	}
	flag.Visit(visitor)

	expandedPath := expandPath(flags.Config)

	if configData, err := ioutil.ReadFile(expandedPath); err != nil {
		if settings.Config != "" {
			// User explicitly asked for a config file and it wasn't there; fail hard.
			log.Fatal(err)
		} else {
			// Default config file missing; just warn.
			log.Print(err)
		}
	} else {
		if err = json.Unmarshal(configData, &config); err != nil {
			log.Fatal(err)
		}
	}

	// Final merge into settings object.
	if settings.Address == "" {
		if config.Address != "" {
			settings.Address = config.Address
		} else {
			settings.Address = defaults.Address
		}
	}
	if settings.Logfile == "" {
		if config.Logfile != "" {
			settings.Logfile = config.Logfile
		} else {
			settings.Logfile = defaults.Logfile
		}
	}
	if settings.Port != 0 || config.Port != 0 {
		if isPath(settings.Address) {
			log.Print("--port option ignored when listening on UNIX domain socket")
		}
	}
	if settings.Port == 0 {
		if config.Port != 0 {
			settings.Port = config.Port
		} else {
			settings.Port = defaults.Port
		}
	}
	if settings.Executable == "" {
		if config.Executable != "" {
			settings.Executable = config.Executable
		} else {
			settings.Executable = defaults.Executable
		}
	}
	if settings.Flags == "" {
		if config.Flags != "" {
			settings.Flags = config.Flags
		} else {
			settings.Flags = defaults.Flags
		}
	}
}

func main() {
	syscall.Umask(0077)
	// Set this up before we even know where our logfile is, in case we have to
	// bail early and print something to stderr.
	log.SetPrefix("clipper: ")
	// Set default values per GOOS.
	setDefaults()
	// Setup flags subsystem.
	initFlags()

	flag.Parse()
	if flag.NArg() != 0 {
		// Additional command-line options not supported.
		flag.Usage()
		os.Exit(1)
	}
	if showHelp {
		printVersion()
		flag.Usage()
		os.Exit(0)
	}
	if showVersion {
		printVersion()
		os.Exit(0)
	}

	// Merge flags -> config -> defaults.
	mergeSettings()

	expandedPath := expandPath(settings.Logfile)
	outfile, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()
	log.SetOutput(outfile)

	if _, err := exec.LookPath(settings.Executable); err != nil {
		log.Fatal(err)
	}

	var addr string
	var listeners []net.Listener
	if isPath(settings.Address) {
		addr = expandPath(settings.Address)
	} else {
		addr = settings.Address
	}
	if strings.HasPrefix(addr, "/") {
		log.Print("Starting UNIX domain socket server at ", addr)
		listeners = append(listeners, listen("unix", addr, -1))
	} else {
		if addr == "" {
			log.Print("Starting TCP server on loopback interface")
			listeners = append(listeners, listen("tcp4", "127.0.0.1", settings.Port))
			listeners = append(listeners, listen("tcp6", "[::1]", settings.Port))
		} else {
			log.Print("Starting TCP server on ", addr)
			listeners = append(listeners, listen("tcp", settings.Address, settings.Port))
		}
	}

	for i := range listeners {
		defer listeners[i].Close()
		go func(listener net.Listener) {
			for {
				conn, err := listener.Accept()
				if err != nil {
					log.Print(err)
					return
				}

				go handleConnection(conn)
			}
		}(listeners[i])
	}

	// Need to catch signals in order for `defer`-ed clean-up items to run.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)
	sig := <-c
	log.Print("Got signal ", sig)
}

func listen(listenType string, addr string, port int) net.Listener {
	if port >= 0 {
		addr = fmt.Sprintf("%s:%d", addr, port)
	}
	listener, err := net.Listen(listenType, addr)
	if err != nil {
		log.Fatal(err)
	}
	return listener
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

	var args []string
	if settings.Flags != "" {
		whitespace := regexp.MustCompile("\\s+")
		args = whitespace.Split(strings.TrimSpace(settings.Flags), -1)
	}
	cmd := exec.Command(settings.Executable, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		log.Printf("[ERROR] pipe init: %v\n", err)
		return
	}

	if err = cmd.Start(); err != nil {
		log.Printf("[ERROR] process start: %v\n", err)
		return
	}

	if copied, err := io.Copy(stdin, conn); err != nil {
		log.Printf("[ERROR] pipe copy: %v\n", err)
	} else {
		log.Print("Echoed ", copied, " bytes")
	}
	stdin.Close()

	if err = cmd.Wait(); err != nil {
		log.Printf("[ERROR] wait: %v\n", err)
	}
}
