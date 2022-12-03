package web

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/pandada8/ic99-web/pkg/charger"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := wsupgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("Failed to set websocket upgrade: %+v", err)
		return
	}
	// TODO: subscribe to charger status
	info, close := charger.Subscribe()
	inboundChan := make(chan []byte, 10)

	go func() {
		for {
			t, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("error when read", err)
				return
			}
			switch t {
			case websocket.TextMessage:
				inboundChan <- msg
			}
		}
	}()

	defer close()
	for {
		select {
		case data := <-info:
			buf, err := json.Marshal(data)
			if err != nil {
				fmt.Printf("Failed to marshal data: %+v", err)
				continue
			}
			err = conn.WriteMessage(websocket.TextMessage, buf)
			if err != nil {
				return
			}
		case msg := <-inboundChan:
			fmt.Println("Received message: ", string(msg))
			// TODO: handle message
		}
	}
}
