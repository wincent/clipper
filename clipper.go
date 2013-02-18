package main

import(
	"log"
	"net"
)

const(
	RECV_BUF_LEN = 1024
)

func main() {
	log.Print("Starting the server")
	listener, err := net.Listen("tcp", "127.0.0.1:8377")
	if err != nil {
		log.Fatal("Listen: ", err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print("Accept: ", err.Error())
			return
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	buf := make([]byte, RECV_BUF_LEN)
	n, err := conn.Read(buf)
	if err != nil {
		log.Print("Read: ", err.Error())
		return
	}

	log.Print("Received ", n, " bytes of data")
	_, err = conn.Write(buf)
	if err != nil {
		log.Print("Write: ", err.Error())
		return
	} else {
		log.Print("Reply echoed")
		conn.Close()
	}
}
