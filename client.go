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
	Hub *Hub

  // The client identifier
  ClientId string

	// The websocket connection.
	Conn *websocket.Conn

	// Buffered channel of outbound messages.
	Send chan []byte
}

// readPump pumps messages from the websocket connection to HandleMessage.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
    HandleMessage(c, message)
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
		c.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
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
  client := &Client{ClientId: initMsg.ClientId, Hub: hub, Conn: conn, Send: make(chan []byte, 256)}

	client.Hub.register <- client

  // welcome to the app
  velcomen, _ := MakeMessage("INIT", nil)
  HandleMessage(client, velcomen)

	// Allow collection of memory referenced by the caller
  // by doing all work in new goroutines.
	go client.writePump()
	go client.readPump()
}


// Handles the message, including sending an error if required
func HandleMessage(client *Client, msg []byte) {
  if len(msg) < 32 {
    SendError(client, errors.New("Invalid message length, must be > 31"))
    return
  }

  header := string(bytes.TrimSpace(msg[:32]))
  fmt.Println("Processing message: ", header)

  switch header {
  case "INIT":
    // send the games
    gls, err := game.AllGames()
    if err != nil {
      SendError(client, err)
      return
    }

    err = MarshalAndSend(client, "GAMES", gls, false)
    if err != nil {
      SendError(client, err)
      return
    }

  case "GAME_REQ":
    req := struct { Action, GameId string }{}

    err := json.Unmarshal(msg[32:], &req)
    if err != nil {
      SendError(client, err)
      return
    }

    switch req.Action {
    case "CREATE":
      // this player will be the host
      g := game.CreateGame(client.ClientId)
      err := MarshalAndSend(client, "START_WAIT", g, false)
      if err != nil {
        SendError(client, err)
        return
      }

      // broadcast the new game list
      gls, err := game.AllGames()
      if err != nil {
        SendError(client, err)
        return
      }

      err = MarshalAndSend(client, "GAMES", gls, true)
      if err != nil {
        SendError(client, err)
        return
      }

      return
    case "JOIN":
      // this player will be the host
      g, err := game.JoinGame(req.GameId, client.ClientId)
      if err != nil {
        SendError(client, err)
        return
      }

      err = MarshalAndSendToGame(client, g, "START_WAIT", g)
      if err != nil {
        SendError(client, err)
        return
      }

      // broadcast the new game list
      gls, err := game.AllGames()
      if err != nil {
        SendError(client, err)
        return
      }

      err = MarshalAndSend(client, "GAMES", gls, true)
      if err != nil {
        SendError(client, err)
        return
      }
    case "LEAVE":
      g, err := game.LeaveGame(req.GameId, client.ClientId)
      if err != nil {
        SendError(client, err)
        return
      }

      err = MarshalAndSendToGame(client, g, "START_WAIT", g)
      if err != nil {
        SendError(client, err)
        return
      }

      // broadcast the new game list
      gls, err := game.AllGames()
      if err != nil {
        SendError(client, err)
        return
      }

      err = MarshalAndSend(client, "GAMES", gls, true)
      if err != nil {
        SendError(client, err)
        return
      }
    }

  case "BEGIN_GAME":
    // we should have a question count
    body := struct { 
      GameId string 
      QuestionCount uint8 
    }{}

    err := json.Unmarshal(msg[32:], &body)
    if err != nil {
      SendError(client, err)
    }

    // update the game with the question count and start round
    g, err := game.UpdateQuestionCount(body.GameId, body.QuestionCount)
    if err != nil {
      SendError(client, err)
    }

    err = MarshalAndSendToGame(client, g, "START_ROUND", g)
    if err != nil {
      SendError(client, err)
      return
    }

    // broadcast the new game list
    gls, err := game.AllGames()
    if err != nil {
      SendError(client, err)
      return
    }

    err = MarshalAndSend(client, "GAMES", gls, true)
    if err != nil {
      SendError(client, err)
      return
    }

  case "GAMEPLAY":
    // unmarshal the message to get the gameplay req type
    reqPart := struct { Request string `json:"req"`
    GameId string `json:"gameId"`}{}
	
	err := json.Unmarshal(msg[32:], &reqPart)
	if err != nil {
		SendError(client, err)
	}
	switch reqPart.Request {
	  case "WHEEL_SPIN":
		reqFull := struct { Request string `json:"req"`
							GameId string `json:"gameId"`
							SpinValue float32 `json:"spinValue"`}{}
		err := json.Unmarshal(msg[32:], &reqFull)
		if err != nil {
			SendError(client, err)
		}
			
		// create a wheel spun message and send it to the other clients
		// doesn't need any handling from game package
		
		spinFwd := struct { GameId string `json:"gameId"`
							PlayerId string `json:"playerId"`
							SpinValue float32 `json:"spinValue"`}{}	
		spinFwd.GameId = reqFull.GameId
		spinFwd.PlayerId = client.ClientId
		spinFwd.SpinValue = reqFull.SpinValue
		
		fmt.Printf("%+v\n", spinFwd)
		
		err = MarshalAndSend(client, "WHEEL_SPUN", spinFwd, true)
        if err != nil {
          SendError(client, err)
          return
        }
		
	  case "QUESTION_SELECT":
		reqFull := struct { Request string `json:"req"`
							GameId string `json:"gameId"`
							Category string 	`json:"category"`
							PointValue uint8	`json:"pointValue"`}{}
		err := json.Unmarshal(msg[32:], &reqFull)
		if err != nil {
			SendError(client, err)
		}
		
		// get the question and send it back to everyone
		q, err := game.QuestionSelect(reqFull.GameId,
									  reqFull.Category,
									  reqFull.PointValue)
		if err != nil {
			SendError(client, err)
		}
		
		// send question to everyone
		err = MarshalAndSend(client, "QUESTION_RESPONSE", q, true)
        if err != nil {
          SendError(client, err)
          return
        }
		
	  case "BUZZ":
		reqFull := struct { Request string `json:"req"`
							GameId string `json:"gameId"`
							Delay uint32		`json:"delay"`
							Expired bool		`json:"expired"`}{}
		err := json.Unmarshal(msg[32:], &reqFull)
		if err != nil {
			SendError(client, err)
		}
		
		// TODO: register the buzz, if this is the third buzz then choose
		// the current player and send the question to everyone
		
		choosePlayer, err := game.RegisterBuzz(reqFull.GameId, 
											   client.ClientId, 
											   reqFull.Delay, 
											   reqFull.Expired)
		if err != nil {
			SendError(client, err)
		}
		
		if choosePlayer {
			
			expired, newCurrPlayerId, err := game.GetNewCurrentPlayer(reqFull.GameId)
			if expired {
				// call IncomingAnswer with no cliendId
				correct, correctAnswer, gls, err := game.IncomingAnswer(reqFull.GameId,
																		"xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
																		0)
				if err != nil {
					SendError(client, err)
				}
				
				// send answer response message
				answerResp := struct { Correct bool `json:"correct"`
									   CorrectAnswer string `json:"correctAnswer"`
									   Game *game.Game `json:"game"`}{}				
				answerResp.Correct = correct
				answerResp.CorrectAnswer = correctAnswer
				answerResp.Game = gls
				
				err = MarshalAndSend(client, "ANSWER_RESPONSE", answerResp, true)
				if err != nil {
				  SendError(client, err)
				  return
				}
				
			} else {
				// send player selected message
				playerSelect := struct { GameId string `json:"gameId"`
										 PlayerId string `json:"playerId"`}{}
				playerSelect.GameId = reqFull.GameId
				playerSelect.PlayerId = newCurrPlayerId
				
				err = MarshalAndSend(client, "PLAYER_SELECTED", playerSelect, true)
				if err != nil {
				  SendError(client, err)
				  return
				}
			}
		
		} // else, do nothing
		
	  case "ANSWER":
		reqFull := struct { Request string `json:"req"`
							GameId string `json:"gameId"`
							AnswerIndex uint8	`json:"answerIndex"`}{}
		err := json.Unmarshal(msg[32:], &reqFull)
		if err != nil {
			SendError(client, err)
		}
		
		// TODO: determine if the answer was correct and then send the
		// answer message back to everyone
		// call IncomingAnswer with no cliendId
		correct, correctAnswer, gls, err := game.IncomingAnswer(reqFull.GameId,
																client.ClientId,
																reqFull.AnswerIndex)
		if err != nil {
			SendError(client, err)
		}
		
		// send answer response message
		answerResp := struct { Correct bool `json:"correct"`
							   CorrectAnswer string `json:"correctAnswer"`
							   Game *game.Game `json:"game"`}{}				
		answerResp.Correct = correct
		answerResp.CorrectAnswer = correctAnswer
		answerResp.Game = gls
		
		err = MarshalAndSend(client, "ANSWER_RESPONSE", answerResp, true)
		if err != nil {
		  SendError(client, err)
		  return
		}
		
		if gls.State == game.ENDED {
			game.RemoveGame(gls.GameId)
		}
	  default:
	    fmt.Println("unknown req type")
	}
  default:
    SendError(client, fmt.Errorf("Unknown message header %q", header))
  }

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

func MarshalAndSendToGame(client *Client, g *game.Game, header string, body interface{}) (error) {
  // TODO: impl
  for _, p := range g.Players {
    err := MarshalAndSend(client.Hub.clients[p.PlayerId], header, body, false)
    if err != nil {
      return err
    }
  }

  return nil
}

func MarshalAndSend(client *Client, header string, body interface{}, broadcast bool) (error) {
  
  fmt.Println("Sending message: ", header)
      // send the start wait message
      mbytes, err := json.Marshal(body)

      if err != nil {
        return err
      }
	  fmt.Println(mbytes)

      msg, err := MakeMessage(header, mbytes)

      if (broadcast) {
        client.Hub.broadcast <- msg
        return nil
      }

      client.Send <- msg
      return nil
}

func SendError(client *Client, err error) {
  client.Send <- []byte(fmt.Sprintf("An error occured: %v", err))
}
