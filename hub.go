package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"strconv"
	"strings"
	"sync"
)

var gameMapMutex = &sync.Mutex{}

type Vertex struct {
	X, Y int
}

type Zombie struct {
	sync.Mutex
	pos *Vertex
}

type State struct {
	zombieChan chan *Zombie
	gameInfo   chan string
	score      *Score
}

type Score struct {
	sync.Mutex
	client int
	server int
}

type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	shotChan chan *Vertex
}

type Hub struct {
	games      map[*Client]*State
	register   chan *Client
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		games:      make(map[*Client]*State),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			gameMapMutex.Lock()
			h.games[client] = &State{
				zombieChan: make(chan *Zombie, 1024),
				gameInfo:   make(chan string),
				score:      &Score{client: 0, server: 0},
			}
			gameMapMutex.Unlock()
			go client.writePump() // route zombies to client
			go client.readPump()  // route shots to game

		case client := <-h.unregister:
			fmt.Println("client unregister ", client)
			gameMapMutex.Lock()
			delete(h.games, client)
			gameMapMutex.Unlock()
		}

	}
}

func handleClientMessage(c *Client, message []byte) {
	if string(message) == "start" {
		fmt.Println("starting game for ", c)
		gameMapMutex.Lock()
		go startZombie(c.hub.games[c], c)
		gameMapMutex.Unlock()
	} else {
		shot := strToVertex(message)
		c.shotChan <- shot

	}
	fmt.Println("got a message: " + string(message))
}

// run readPump per connection
func (c *Client) readPump() {
	defer func() {
		fmt.Println("closing connection and sending unregister Read Pump")
		c.hub.unregister <- c
		c.conn.Close()
	}()

ListenForMessages:
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Println("connection lost. exiting read loop")
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				fmt.Printf("error: %v", err)
			}
			break ListenForMessages
		}
		handleClientMessage(c, message)

	}
}

func (c *Client) writePump() {
	defer func() {
		fmt.Println("closing connection and sending unregister. Write Pump")
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			fmt.Println("write close(closed by other end):", err)
			return
		}
		c.hub.unregister <- c
		c.conn.Close()
	}()

	gameMapMutex.Lock()
	game := c.hub.games[c]
	gameMapMutex.Unlock()

WriteLoop:
	for {
		select {
		case zombieMove, ok := <-game.zombieChan:
			if !ok {
				fmt.Println("zombie chan closed, stop listening")
			} else {
				w, err := c.conn.NextWriter(websocket.TextMessage)
				if err != nil {
					fmt.Println(err)
					return // stay in loop and keep writing from game.info chan
				}
				zombieMove.Lock()
				msg := XYToStr(zombieMove.pos)
				zombieMove.Unlock()
				w.Write([]byte(msg))
				if err := w.Close(); err != nil { // not sure if I should close writer each time here
					fmt.Println(err)
				}
			}
		case info, ok := <-game.gameInfo:
			if !ok {
				fmt.Println("info chan closed")
				break WriteLoop
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				fmt.Println("websocket write failed")
				fmt.Println(err)
				break WriteLoop
			}
			w.Write([]byte(info))
			if err := w.Close(); err != nil {
				fmt.Println("websocket close failed")
				fmt.Println(err)
			}
		}
	}
}

// conversion helpers
func strToVertex(s []byte) *Vertex {
	strPos := strings.Split(string(s), " ")
	x, err := strconv.Atoi(strPos[0])
	if err != nil {
		fmt.Errorf("failed to convert coordinates")
	}
	y, erry := strconv.Atoi(strPos[1])
	if erry != nil {
		fmt.Errorf("failed to convert coordinates")
	}
	return &Vertex{X: x, Y: y}
}
func XYToStr(pos *Vertex) (s string) {
	s = string(strconv.Itoa(pos.X) + " " + strconv.Itoa(pos.Y))
	return
}
