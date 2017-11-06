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
	go hub.run()

	http.HandleFunc("/websocket", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		fmt.Println("ListenAndServe: ", err)
	}

}

func startZombie(state *State, client *Client) {
	ticker := time.NewTicker(time.Second * 2)
	zombieAt := &Zombie{pos: &Vertex{0, 10}}
	defer func() {
		ticker.Stop()
		close(state.zombieChan)
		fmt.Println("zombie routine end, clock stopped")
	}()
	gameInfo := "started"

ZombieLifecycle:
	for {
		select {
		case shot := <-client.shotChan:
			zombieAt.Lock()
			var isAHit bool = (zombieAt.pos.X == shot.X && zombieAt.pos.Y == shot.Y)
			zombieAt.Unlock()
			if isAHit {
				gameInfo = "zombie got hit. Client wins"
				break ZombieLifecycle
			}
		case <-ticker.C:
			zombieAt.Lock()
			pos := zombieAt.pos
			if pos.X >= GAME_WIDTH {
				zombieAt.Unlock()
				gameInfo = "Zombie reaches the end. Client loose"
				break ZombieLifecycle

			}
			pos.X += 1
			pos.Y = getRandomFromCenter(pos.Y)
			zombieAt.pos = pos
			state.zombieChan <- zombieAt
			zombieAt.Unlock()
		case <-client.terminator:
			gameInfo = "Client is lost."
			break ZombieLifecycle
		}
	}
	state.gameInfo <- gameInfo
	//state.close(gameInfo)
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
