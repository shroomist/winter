package main

import (
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"
)

var addr = flag.String("addr", "localhost:3000", "http service address")

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/websocket"}
	log.Printf("connecting to %s", u.String())
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()
	done := make(chan struct{})

	go func() {
		defer c.Close()
		defer close(done)
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", msg)
		}
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	otherticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 3; i++ {
		gamestarterr := c.WriteMessage(websocket.TextMessage, []byte("start"))
		if gamestarterr != nil {
			log.Println("write:", err)
			return
		}

	}

	for {
		select {
		case <-otherticker.C:
			gamestarterr := c.WriteMessage(websocket.TextMessage, []byte("start"))
			log.Println("start")
			if gamestarterr != nil {
				log.Println("write:", err)
				return
			}
		case <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(strings.Join([]string{strconv.Itoa(1 + rand.Intn(3)), " ", strconv.Itoa(10 - rand.Intn(3))}, "")))
			if err != nil {
				//log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")
			// To cleanly close a connection, a client should send a close
			// frame and wait for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			c.Close()
			return
		}
	}
}
