package main

import(
	"net"
	"os"
)

const(
	RECV_BUF_LEN = 1024
)

func main() {
	println("Starting the server")
	listener, err := net.Listen("tcp", "127.0.0.1:8377")
	if err != nil {
		println("error Listen: ", err.Error())
		os.Exit(1)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			println("error Accept: ", err.Error())
			return
		}

		go HandleConnection(conn)
	}
}

func HandleConnection(conn net.Conn) {
	buf := make([]byte, RECV_BUF_LEN)
	n, err := conn.Read(buf)
	if err != nil {
		println("error Read: ", err.Error())
		return
	}
	println("Received ", n, " bytes of data: ", string(buf))
	_, err = conn.Write(buf)
	if err != nil {
		println("error Write: ", err.Error())
		return
	} else {
		println("Reply echoed")
	}
}
