package main

import(
	"io"
	"log"
	"net"
	"os/exec"
)

func main() {
	_, err := exec.LookPath("pbcopy")
	if err != nil {
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
	copied, err := io.Copy(conn, conn)
	if err != nil {
		log.Print(err.Error())
	} else {
		log.Print("Echoed ", copied, " bytes")
	}
	conn.Close()
	log.Print("Connection closed")
}
