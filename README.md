# Winter is Coming via websocket

## Dependencies
```
go get github.com/gorilla/websocket
```

## Run Server
```
go run *.go
```
server accepts 2 kinds of messages, its either
`start` which spawns a zombie or 
`X Y` space delimited int coordinates for shooting an arrow 
(no error handling on casting this str to int)

## Run Client
```
cd client
go run client.go
```
Currently client spawns 3 zombies on start
and then spawns a zombie each 0.5 sec
he's sending arrows randomly


## quirks
Zombies would still run and count score even when the client is lost.
