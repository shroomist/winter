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
	zombieAt   *Zombie
	terminator chan bool
	zombieChan chan *ShotOrZombie
	shotChan   chan *ShotOrZombie
}

func (state *State) close() {
	fmt.Println("sending closing game signals")
	state.terminator <- true
	close(state.zombieChan) // perhaps these get closed down the line
	close(state.shotChan)
	close(state.terminator)
}

type Client struct {
	sid  string
	hub  *Hub
	conn *websocket.Conn
	send chan []byte //sending zombies via this channel
}

type ShotOrZombie struct {
	client *Client // this is perhaps unnecessary
	pos    *Vertex
}

type Hub struct {
	games      map[*Client]*State
	register   chan *Client
	unregister chan *Client
	shots      chan *ShotOrZombie
}

func newHub() *Hub {
	return &Hub{
		games:      make(map[*Client]*State),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shots:      make(chan *ShotOrZombie),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.games[client] = &State{ // RACE prev w
				zombieAt:   &Zombie{pos: &Vertex{1, 10}}, //#RACE 1 previous write #RACE 2 3 prev write
				terminator: make(chan bool),
				zombieChan: make(chan *ShotOrZombie),
				shotChan:   make(chan *ShotOrZombie), //#RACE 2 4
			}
		case client := <-h.unregister:
			fmt.Println("client unregister ", client)
			h.games[client].close()
			delete(h.games, client)
		case shot := <-h.shots:
			select {
			//case shot := <-h.games[shot.client].shotChan:
			//fmt.Println("got shot at ", shot)
			default:
				fmt.Println("Fallthru to closing the game", shot)
				//h.games[shot.client].close()

			}
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
			c.hub.games[c].shotChan <- &ShotOrZombie{
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
		select {
		case zombieMove, ok := <-c.hub.games[c].zombieChan: // #RACE, game wasnt created
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil || !ok {
				return // more err handle
			}
			fmt.Println(zombieMove)
			w.Write([]byte(strconv.Itoa(zombieMove.pos.X) + " " + strconv.Itoa(zombieMove.pos.Y)))
		}
	}

}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) { //#RACE
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump() // #RACE
	go client.readPump()
}
