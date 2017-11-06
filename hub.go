package main

import (
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Zombie struct {
	sync.Mutex
	pos *Vertex
}

type State struct {
	zombieChan chan *Zombie
	gameInfo   chan string
}

func (state *State) close(gi string) {
	state.gameInfo <- gi
	//close(state.zombieChan)
}

type Client struct {
	sid        string
	hub        *Hub
	conn       *websocket.Conn
	shotChan   chan *Vertex
	terminator chan bool
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

var gameMapMutex = &sync.Mutex{}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			gameMapMutex.Lock()
			h.games[client] = &State{
				zombieChan: make(chan *Zombie),
				gameInfo:   make(chan string),
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

var upgrader = websocket.Upgrader{ // websocket defaults
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// run readPump per connection
func (c *Client) readPump() {
	defer func() {
		fmt.Println("closing connection and sending unregister Read Pump")
		c.terminator <- true
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
		if string(message) == "start" {
			startGame(c)
		} else {
			x, y := strToXY(message)
			c.shotChan <- &Vertex{X: x, Y: y}

		}
		fmt.Println("got a message: " + string(message))
	}

	fmt.Println("end of read pump")
}

func strToXY(s []byte) (x int, y int) {
	strPos := strings.Split(string(s), " ")
	x, err := strconv.Atoi(strPos[0])
	if err != nil {
		fmt.Errorf("failed to convert coordinates")
	}
	y, erry := strconv.Atoi(strPos[1])
	if erry != nil {
		fmt.Errorf("failed to convert coordinates")
	}
	return
}
func XYToStr(x int, y int) (s string) {
	s = string(strconv.Itoa(x) + " " + strconv.Itoa(y))
	return
}

func (c *Client) writePump() {
	defer func() {
		fmt.Println("closing connection and sending unregister. Write Pump")
		err := c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		if err != nil {
			fmt.Println("write close(closed by other end):", err)
			return
		}
		//c.terminator <- true
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
				game.zombieChan = nil
				break // stay in loop and keep writing from game.info chan
			} else {
				w, err := c.conn.NextWriter(websocket.TextMessage)
				if err != nil {
					fmt.Println(err)
					return // stay in loop and keep writing from game.info chan
				}
				zombieMove.Lock()
				msg := XYToStr(zombieMove.pos.X, zombieMove.pos.Y)
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
	fmt.Println("end of write pump")

}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) { //#RACE
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, shotChan: make(chan *Vertex), terminator: make(chan bool)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
}
