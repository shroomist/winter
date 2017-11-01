package main

import (
	"flag"
	"net/http"
	//"./common"
	"./util"
	"fmt"
	//"io/ioutil"
	"math/rand"
	//"os"
	"sync"
	"time"
)

var mutex = &sync.Mutex{}

//var winterHttpServer = util.WinterHttpServer
type Vertex struct {
	X, Y int
}

var addr = flag.String("addr", ":3000", "http service address")

const GAME_WIDTH int = 3

const GAME_HEIGHT int = 20

func init() {
	rand.Seed(time.Now().UnixNano()) //#1

}

var winterHttpInst util.WinterHttpServer

func main() {
	hub := newHub()
	go hub.run() //#RACE

	http.HandleFunc("/websocket", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r) // #RACE
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}

}

func startZombie(state *State, client *Client) {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()

	zombieAt := &Zombie{pos: &Vertex{0, 10}}

ZombieLifecycle:
	for {
		select {
		case shot := <-client.shotChan: //#RACE 2!
			zombieAt.Lock()
			var isAHit bool = (zombieAt.pos.X == shot.pos.X && zombieAt.pos.Y == shot.pos.Y)
			zombieAt.Unlock()
			if isAHit {
				state.close()
				fmt.Println("zombie got hit")
				break ZombieLifecycle
			}
		case <-ticker.C:
			zombieAt.Lock()
			pos := zombieAt.pos
			if pos.X >= GAME_WIDTH { //#RACE 3 read
				zombieAt.Unlock()
				state.close()
				fmt.Println("Position overlap zombie wins")
				break ZombieLifecycle

			}
			pos.X += 1
			pos.Y = getRandomFromCenter(pos.Y) //#RACE 2 read
			zombieAt.pos = pos
			state.zombieChan <- zombieAt //#RACE 2 routine created
			zombieAt.Unlock()
		case <-state.terminator: // #RACE 4
			fmt.Println("exiting zombie routine")
			break ZombieLifecycle
		}
	}
	fmt.Println("zombie routine end")
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

func startGame(client *Client) {
	fmt.Println("starting game for ", client)
	go startZombie(client.hub.games[client], client) //#RACE 1 3 created
}
