package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	GAME_WIDTH            = 3
	GAME_HEIGHT           = 20
	ZOMBIE_CLOCK          = 3 // seconds
	WEBSOCKET_BUFFER_SIZE = 1024
)

var addr = flag.String("addr", ":3000", "http service address")

var upgrader = websocket.Upgrader{ // websocket defaults
	ReadBufferSize:  WEBSOCKET_BUFFER_SIZE,
	WriteBufferSize: WEBSOCKET_BUFFER_SIZE,
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/websocket", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	fmt.Println("Listening for client connections")
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		fmt.Println("ListenAndServe err: ", err)
	}

}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, shotChan: make(chan *Vertex)}
	client.hub.register <- client
}

func startZombie(state *State, client *Client) {
	ticker := time.NewTicker(time.Second * ZOMBIE_CLOCK)
	zombieAt := &Zombie{pos: &Vertex{0, GAME_HEIGHT / 2}}
	gameInfo := "started"
	defer func() {
		ticker.Stop()
		fmt.Println("zombie routine end, clock stopped. ", gameInfo)
		state.gameInfo <- gameInfo
	}()

ZombieLifecycle:
	for {
		select {
		case shot := <-client.shotChan:
			zombieAt.Lock()
			var isAHit bool = (zombieAt.pos.X == shot.X && zombieAt.pos.Y == shot.Y)
			zombieAt.Unlock()
			if isAHit {
				state.score.Lock()
				state.score.client++
				gameInfo = strings.Join([]string{"zombie got hit ", strconv.Itoa(state.score.client), ":", strconv.Itoa(state.score.server)}, "")
				state.score.Unlock()
				break ZombieLifecycle
			}
		case <-ticker.C:
			zombieAt.Lock()
			pos := zombieAt.pos
			if pos.X >= GAME_WIDTH {
				zombieAt.Unlock()

				state.score.Lock()
				state.score.server++
				gameInfo = strings.Join([]string{"client dead ", strconv.Itoa(state.score.client), ":", strconv.Itoa(state.score.server)}, "")
				state.score.Unlock()
				break ZombieLifecycle

			}
			pos.X += 1
			pos.Y = getRandomFromCenter(pos.Y)
			zombieAt.pos = pos
			select { // this select is a non blocking send on a channel
			case state.zombieChan <- zombieAt: // it sent
			default:
				fmt.Println("sending zombie pos is being blocked, client lost or write pump is slow")
				break ZombieLifecycle
			}
			zombieAt.Unlock()

		}
	}
}

func getRandomFromCenter(c int) int {
	switch {
	case c == 0:
		return rand.Intn(2)
	case c == GAME_HEIGHT:
		return c - rand.Intn(2)
	default:
		return c + 1 - rand.Intn(3)

	}
}
