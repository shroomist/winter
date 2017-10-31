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

const GAME_WIDTH int = 30

const GAME_HEIGHT int = 20

//var state = make(map[string]*State)

func gameEnd(state *State, client *Client) {
	state.terminator <- true

	close(state.zombieChan)
	close(state.shotChan)
	//killGame(state, client)
}

//func killGame(state *State, client *Client) {
//close(state[sid].zombieChan)
//close(state[sid].shotChan)
//delete(state, sid)
//}

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
	//winterHttpInst.Init(startGame)
	//listenAndServeHttpClient()

}

func listenForPositions(zombiePos <-chan *Vertex) {
	for zp := range zombiePos {
		fmt.Println("zombie at %+v", zp)
	}
}
func listenForShots(shots <-chan *Vertex, sid string) {
	for shot := range shots {
		fmt.Println("shot at ", shot)
		//mutex.Lock()
		//fmt.Println("state of session", state[sid].ZombieAt)
		//mutex.Unlock()
	}
}
func emitShots(sc chan<- *Vertex) {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for _ = range ticker.C {
		sc <- &Vertex{rand.Intn(20), rand.Intn(20)}
	}

}

func startZombie(state *State, client *Client) {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for {
		select {
		case shot := <-state.shotChan: //#RACE 2!
			state.zombieAt.Lock()
			var isAHit bool = state.zombieAt.pos.X == shot.pos.X && state.zombieAt.pos.Y == shot.pos.Y
			state.zombieAt.Unlock()
			if isAHit {
				state.terminator <- true
			}
			fmt.Println("catching shot at ", shot.pos.X, shot.pos.Y)
		case <-ticker.C:
			pos := state.zombieAt.pos // race reade
			if pos.X >= GAME_WIDTH {  //#RACE 3 read
				gameEnd(state, client)
				return
			}
			pos.X += 1
			pos.Y = getRandomFromCenter(pos.Y) //#RACE 2 read
			state.zombieAt.Lock()              //#RACE 1 Write
			state.zombieAt.pos = pos
			state.zombieAt.Unlock()
			state.zombieChan <- &ShotOrZombie{pos: pos, client: client} //#RACE 2 routine created
		case <-state.terminator: // #RACE 4
			fmt.Println("exiting zombie routine")
			return
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

	//token = util.RandStringRunes(16) //pass rand object here? #1
	//state[token] = &State{ZombieAt: &Vertex{0, GAME_HEIGHT / 2}}

	//zombiePos := make(chan *Vertex)
	//archerShots := make(chan *Vertex)
	//terminator = make(chan bool)
	fmt.Println("starting game for ", client)

	go startZombie(client.hub.games[client], client) //#RACE 1 3 created
	//go listenForPositions(zombiePos)
	//go listenForShots(archerShots, token)
	//go emitShots(archerShots)

	//state[token].zombieChan = zombiePos
	//state[token].shotChan = archerShots

	//return
}
