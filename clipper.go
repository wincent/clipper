package main

import(
	"io"
	"log"
	"net"
	"os/exec"
)

const(
	PBCOPY = "pbcopy"
)

func main() {
	if _, err := exec.LookPath(PBCOPY); err != nil {
		log.Fatal(err.Error())
	}

	log.Print("Starting the server")
	listener, err := net.Listen("tcp", "127.0.0.1:8377")
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
