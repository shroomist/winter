package main

import (
	//"./common/"
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
	//zombieAt   *Zombie
	terminator chan bool
	zombieChan chan *Zombie
}

func (state *State) close() {
	fmt.Println("sending closing game signals")

	fmt.Println("1")
	close(state.zombieChan) // perhaps these get closed down the line
	fmt.Println("2")
	close(state.terminator)
	fmt.Println("all zombie channels closed")
}

type Client struct {
	sid      string
	hub      *Hub
	conn     *websocket.Conn
	shotChan chan *ShotOrZombie // rename

}

type ShotOrZombie struct {
	client *Client // this is perhaps unnecessary
	pos    *Vertex
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
			h.games[client] = &State{ // RACE prev w
				terminator: make(chan bool),
				zombieChan: make(chan *Zombie),
			}
			gameMapMutex.Unlock()
			go client.writePump() // #RACE
			go client.readPump()

		case client := <-h.unregister:
			fmt.Println("client unregister ", client)
			gameMapMutex.Lock()
			h.games[client].close()
			delete(h.games, client) // RACE
			gameMapMutex.Unlock()
		}

	}
	fmt.Println("Reached end of hub run")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// run readPump per connection
func (c *Client) readPump() {
	defer func() {
		fmt.Println("closing connection and sending unregister")
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				fmt.Printf("error: %v", err)
			}
			break
		}
		//c.hub.shots <- &ShotOrZombie{client: c, pos: &Vertex{30, 39}}
		if string(message) == "start" {
			startGame(c) // #RACE 1 created # RACE 2 created 3
		} else {
			strPos := strings.Split(string(message), " ")
			x, err := strconv.Atoi(strPos[0])
			if err != nil {
				fmt.Errorf("failed to convert coordinates")
			}
			y, erry := strconv.Atoi(strPos[1])
			if erry != nil {
				fmt.Errorf("failed to convert coordinates")
			}
			c.shotChan <- &ShotOrZombie{ // RACE? shall we lock here?
				pos:    &Vertex{X: x, Y: y},
				client: c,
			}

		}
		fmt.Println(string(message))
	}
}

func (c *Client) writePump() {
	// add lost connection handle
	for {
		//select {
		//case zombieMove, ok := <-c.hub.games[c].zombieChan: // #RACE, game wasnt created
		gameMapMutex.Lock()
		zombieMove, ok := <-c.hub.games[c].zombieChan // #RACE, game wasnt created
		gameMapMutex.Unlock()
		w, err := c.conn.NextWriter(websocket.TextMessage)
		if err != nil || !ok {
			return // more err handle
		}
		zombieMove.Lock()
		w.Write([]byte(strconv.Itoa(zombieMove.pos.X) + " " + strconv.Itoa(zombieMove.pos.Y)))
		zombieMove.Unlock()
		//}
	}

}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) { //#RACE
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, shotChan: make(chan *ShotOrZombie)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
}
