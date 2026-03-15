package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	Models "tui/pkg/models"
)

func handleConnection(conn net.Conn) {
	defer conn.Close()

	store := Models.GetStore()

	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		parts := strings.Fields(message)

		switch parts[0] {
		case "LIST_PRODUCTS":
			products := store.GetProductsString()
			conn.Write([]byte(fmt.Sprintf("OK [%v]\n", products)))

		case "ADD_PRODUCTS":

		default:
			conn.Write([]byte("ERROR command not found\n"))

		}

		if err != nil {
			fmt.Println("Connection closed")
			return
		}

		fmt.Print("Received:", message)

		conn.Write([]byte("Message received\n"))
	}
}

func main() {
	listener, err := net.Listen("tcp", ":8080")

	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("TCP Server listening on port 8080")

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go handleConnection(conn)
	}
}
