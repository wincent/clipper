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
	"strconv"
	"strings"
	"syscall"
)

type IntFlag struct {
	provided bool
	value    int
}

// From flag.Value interface.
func (f *IntFlag) Set(s string) error {
	i, err := strconv.Atoi(s)
	f.provided = true
	f.value = i
	if err != nil {
		return err
	}
	return nil
}

// From flag.Value interface.
func (f *IntFlag) String() string {
	return strconv.Itoa(f.value)
}

// From json.Unmarsheler interface.
func (f *IntFlag) UnmarshalJSON(b []byte) error {
	i, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*f = IntFlag{provided: true, value: i}
	return nil
}

type StringFlag struct {
	provided bool
	value    string
}

// From flag.Value interface.
func (f *StringFlag) Set(s string) error {
	f.provided = true
	f.value = s
	return nil
}

// From to flag.Value interface.
func (f *StringFlag) String() string {
	return f.value
}

// From json.Unmarsheler interface.
func (f *StringFlag) UnmarshalJSON(b []byte) error {
	var raw string
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*f = StringFlag{provided: true, value: raw}
	return nil
}

type Options struct {
	Address    StringFlag
	Config     StringFlag
	Logfile    StringFlag
	Executable StringFlag
	Flags      StringFlag
	Port       IntFlag
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
	flag.Var(&flags.Port, "p", listenPortUsage)
	flag.Var(&flags.Port, "port", listenPortUsage)
	flag.Var(&flags.Address, "a", listenAddrUsage)
	flag.Var(&flags.Address, "address", listenAddrUsage)
	flag.Var(&flags.Config, "c", configFileUsage)
	flag.Var(&flags.Config, "config", configFileUsage)
	flag.Var(&flags.Executable, "e", executableUsage)
	flag.Var(&flags.Executable, "executable", executableUsage)
	flag.Var(&flags.Flags, "f", flagsUsage)
	flag.Var(&flags.Flags, "flags", flagsUsage)
	flag.Var(&flags.Logfile, "l", logFileUsage)
	flag.Var(&flags.Logfile, "logfile", logFileUsage)
	flag.BoolVar(&showVersion, "v", false, versionUsage)
	flag.BoolVar(&showVersion, "version", false, versionUsage)
}

func setDefaults() {
	defaults.Address = StringFlag{value: ""} // IPv4/IPv6 loopback.
	defaults.Port = IntFlag{value: 8377}

	if runtime.GOOS == "linux" {
		defaults.Config = StringFlag{value: "~/.config/clipper/clipper.json"}
		defaults.Logfile = StringFlag{value: "~/.config/clipper/logs/clipper.log"}
		defaults.Executable = StringFlag{value: "xclip"}
		defaults.Flags = StringFlag{value: "-selection clipboard"}
	} else {
		defaults.Config = StringFlag{value: "~/.clipper.json"}
		defaults.Logfile = StringFlag{value: "~/Library/Logs/com.wincent.clipper.log"}
		defaults.Executable = StringFlag{value: "pbcopy"}
		defaults.Flags = StringFlag{value: ""}
	}
}

func mergeSettings() {
	flag.Parse()

	var expandedPath string
	if flags.Config.provided {
		expandedPath = expandPath(flags.Config.value)
	} else {
		expandedPath = expandPath(defaults.Config.value)
	}

	if configData, err := ioutil.ReadFile(expandedPath); err != nil {
		if flags.Config.provided {
			// User explicitly asked for a config file and it wasn't there; fail hard.
			log.Fatal(err)
		} else {
			// Default config file unreadable (probably missing); just warn.
			log.Print(err)
		}
	} else {
		if err = json.Unmarshal(configData, &config); err != nil {
			log.Fatal(err)
		}
	}

	// Final merge into settings object.
	if flags.Address.provided {
		settings.Address = flags.Address
	} else if config.Address.provided {
		settings.Address = config.Address
	} else {
		settings.Address = defaults.Address
	}
	if flags.Logfile.provided {
		settings.Logfile = flags.Logfile
	} else if config.Logfile.provided {
		settings.Logfile = config.Logfile
	} else {
		settings.Logfile = defaults.Logfile
		if runtime.GOOS == "linux" {
			configDir := pathByExpandingTildeInPath(filepath.Dir(defaults.Logfile.value))
			os.MkdirAll(configDir, 0700)
		}
	}
	if flags.Port.provided || config.Port.provided {
		if isPath(settings.Address.value) {
			log.Print("--port option ignored when listening on UNIX domain socket")
		}
	}
	if flags.Port.provided {
		settings.Port = flags.Port
	} else if config.Port.provided {
		settings.Port = config.Port
	} else {
		settings.Port = defaults.Port
	}
	if flags.Executable.provided {
		settings.Executable = flags.Executable
	} else if config.Executable.provided {
		settings.Executable = config.Executable
	} else {
		settings.Executable = defaults.Executable
	}
	if flags.Flags.provided {
		settings.Flags = flags.Flags
	} else if config.Flags.provided {
		settings.Flags = config.Flags
	} else {
		settings.Flags = defaults.Flags
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

	expandedPath := expandPath(settings.Logfile.value)
	outfile, err := os.OpenFile(expandedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer outfile.Close()
	log.SetOutput(outfile)

	if _, err := exec.LookPath(settings.Executable.value); err != nil {
		log.Fatal(err)
	}

	var addr string
	var listeners []net.Listener
	if isPath(settings.Address.value) {
		addr = expandPath(settings.Address.value)
	} else {
		addr = settings.Address.value
	}
	if strings.HasPrefix(addr, "/") {
		// Check to see if there is a pre-existing or stale socket present.
		if _, err := os.Stat(addr); !os.IsNotExist(err) {
			// Socket already exists.
			if _, err = net.Dial("unix", addr); err == nil {
				// Socket is live!
				log.Fatal("Live socket already exists at: " + addr)
			}

			// Likely a stale socket left over after a crash.
			log.Print("Dead socket found at: " + addr + " (removing)")
			if err = os.Remove(addr); err != nil {
				log.Fatal(err)
			}
		}

		log.Print("Starting UNIX domain socket server at ", addr)
		listeners = append(listeners, listen("unix", addr, -1))
	} else {
		if addr == "" {
			log.Print("Starting TCP server on loopback interface")
			listeners = append(listeners, listen("tcp4", "127.0.0.1", settings.Port.value))
			listeners = append(listeners, listen("tcp6", "[::1]", settings.Port.value))
		} else {
			log.Print("Starting TCP server on ", addr)
			listeners = append(listeners, listen("tcp", settings.Address.value, settings.Port.value))
		}
	}

	listeners = filter(listeners, func(l net.Listener) bool {
		return l != nil
	})
	if len(listeners) == 0 {
		log.Fatal("Failed to establish a listener")
	}

	for i := range listeners {
		if listeners[i] != nil {
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
		log.Print(err)
	}
	return listener
}

func filter(ls []net.Listener, fn func(net.Listener) bool) []net.Listener {
	var out []net.Listener
	for i := range ls {
		if fn(ls[i]) {
			out = append(out, ls[i])
		}
	}
	return out
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
	if settings.Flags.value != "" {
		whitespace := regexp.MustCompile("\\s+")
		args = whitespace.Split(strings.TrimSpace(settings.Flags.value), -1)
	}
	cmd := exec.Command(settings.Executable.value, args...)
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
