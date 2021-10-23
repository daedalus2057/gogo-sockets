package main

import (
  "fmt"
  "errors"
	"bytes"
	"log"
	"net/http"
	"time"
  "encoding/json"

  "gogo-sockets/game"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512

  secretKey = "293faecad3499bdd836090ffc2a72693954be4c842c5728ae9a2148cf802a9359fe27a6ad42af696d3d3008557c8953f93c3681764edad05aa237932e1cc9d45678e7386f625c8d119595e67ff404312ddcfa642f4a2816fc838dc2ec3924fa044c92a7e0cb2e493519ec18a6d4879a9e091312c58f2bc472aa52dcca955799b"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

  // The client identifier
  clientId string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		c.hub.broadcast <- message
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

  go authAndRegister(hub, conn)
}

func authAndRegister(hub *Hub, conn *websocket.Conn) {
  // get the initial message -- validate it meets our expectations
  // and happens within a few seconds...

  // initial conn settings to be overridden in the client
  conn.SetReadLimit(1024)
  conn.SetReadDeadline(time.Now().Add(3 * time.Second))

  msgFormat, msg, err := conn.ReadMessage()
  if err != nil {
    log.Println("Could not read initial message from client: ", err)
    conn.Close()
    return
  }

  // validate the initial message
  if len(msg) < 32 { // err
    log.Println("Invalid initial message from client (len < 32) ")
    conn.Close()
    return
  }

  if msgFormat != websocket.TextMessage {
    log.Println("Invalid initial message format (expected text got binary)")
    conn.Close()
    return
  }

  msgType := string(bytes.TrimSpace(msg[:32]))

  if msgType != "HELO" {
    log.Println("Invalid initial message type, expected HELO got ", msgType)
    conn.Close()
    return
  }

  initMsg := struct {
    Key string `json:"key"`
    ClientId string `json:"clientId"`
  }{}

  err = json.Unmarshal(msg[32:], &initMsg)
  if err != nil {
    log.Println("Could not unmarshal initial message: ", err)
    conn.Close()
    return
  }

  if initMsg.Key != secretKey {
    log.Println("Invalid key")
    conn.Close()
    return
  }

  // in memory client -- identifed by memory address
  client := &Client{clientId: initMsg.ClientId, hub: hub, conn: conn, send: make(chan []byte, 256)}

	client.hub.register <- client

  // welcome to the app
  velcomen, _ := MakeMessage("INIT", nil)
  HandleMessage(client, velcomen)

	// Allow collection of memory referenced by the caller
  // by doing all work in new goroutines.
	go client.writePump()
	go client.readPump()
}

func MakeMessage(header string, body []byte) ([]byte, error) {
  if len(header) > 32 {
    return nil, errors.New("Invalid header")
  }

  // pad the header
  header = fmt.Sprintf("%-32s", header)
  // prepend the header
  msg := append([]byte(header), body...)

  return msg, nil
}

// Handles the message, including sending an error if required
func HandleMessage(client *Client, msg []byte) {
  if len(msg) < 32 {
    SendError(client, errors.New("Invalid message length, must be > 31"))
    return
  }

  switch header := string(bytes.TrimSpace(msg[:32])); header {
  case "INIT":
    // send the games
    gls, err := game.AllGames()
    if err != nil {
      SendError(client, err)
      return
    }

    gbytes, err := json.Marshal(gls)
    if err != nil {
      SendError(client, err)
      return
    }

    msg, err := MakeMessage("GAMES", gbytes)

    client.send <- msg

  default:
    SendError(client, fmt.Errorf("Unknown message header %q", header))
  }

}

func SendError(client *Client, err error) {
  client.send <- []byte(fmt.Sprintf("An error occured: %v", err))
}
