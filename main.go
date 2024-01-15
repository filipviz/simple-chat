package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"time"
)

func main() {
	listner, err := net.Listen("tcp", "localhost:8000")
	if err != nil {
		log.Fatal(err)
	}

	go broadcaster()

	for {
		conn, err := listner.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleConn(conn)
	}

}

type client struct {
	ch  chan<- string
	who string
}

var (
	entering = make(chan client)
	leaving  = make(chan client)
	messages = make(chan string)
)

func broadcaster() {
	clients := make(map[client]bool)
	for {
		select {
		case msg := <-messages:
			// Broadcast message to all clients
			for cli := range clients {
				select {
				case cli.ch <- msg:
					// attempt to send. messages will build up in the buffer
				default:
					// if their buffer is full, close the connection
					delete(clients, cli)
					close(cli.ch)
				}
			}
		case cli := <-entering:
			clients[cli] = true
			m := "Current members:\n"
			for cli := range clients {
				m += cli.who + "\n"
			}
			cli.ch <- m
		case cli := <-leaving:
			delete(clients, cli)
			close(cli.ch)
		}
	}
}

func handleConn(conn net.Conn) {
	ch := make(chan string, 3) // outgoing messages. Buffer of 3 allows for messaging queue in case of poor latency.
	go clientWriter(conn, ch)

	ch <- "Enter your name:"
	input := bufio.NewScanner(conn)
	var who string
	if input.Scan() {
		who = input.Text()
	} else {
		conn.Close()
		return
	}

	ch <- "You are " + who

	messages <- who + " has arrived"
	entering <- client{ch, who}

	sent := make(chan struct{})
	go func() {
		for {
			select {
			case <-time.After(5 * time.Minute):
				fmt.Fprintln(conn, "Closing connection due to inactivity.") // Ignoring errors
				conn.Close()
				// close the connection
			case <-sent:
				// keep going
			}
		}
	}()

	for input.Scan() {
		messages <- who + ": " + input.Text()
		sent <- struct{}{}
	}
	// Ignoring potential errors from input.Err()

	leaving <- client{ch, who}
	messages <- who + " has left"
	conn.Close()
}

func clientWriter(conn net.Conn, ch <-chan string) {
	for msg := range ch {
		fmt.Fprintln(conn, msg) // Ignoring network errors
	}
}
