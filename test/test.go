package main

import (
  "fmt"
  //"errors"
  "gogo-sockets/game"
  //"time"
  //"bytes"
  "github.com/google/uuid"
  "encoding/json"
  "gogo-sockets/game/questions"
)

func testCreateJoinJoin() bool {

  println("testing game creation, joining a game (x2), and json marshaling and unmarshaling of a game object\n")

  playerId0 := uuid.NewString()
  createdGame := game.CreateGame(playerId0)
  fmt.Printf("createdGame = %+v\n\n", createdGame)
  
  
  playerId1 := uuid.NewString()
  joinedGame, err := game.JoinGame(createdGame.GameId, playerId1)
  
  if joinedGame != createdGame {
    fmt.Println("createdGame and joinGame not the same after two joins")
	return false
  } else if joinedGame.State != 0 {
    fmt.Printf("unexpected state %d, expected %d", joinedGame.State, game.WAITING)
	return false
  } else if err != nil {
    fmt.Println("joinGame failed: ", err)
	return false
  } else {
    fmt.Printf("joinGame successful, game = %+v\n\n", joinedGame)
  }
  
  
  
  playerId2 := uuid.NewString()
  joinedGame, err = game.JoinGame(createdGame.GameId, playerId2)
  
  if joinedGame != createdGame {
    fmt.Println("createdGame and joinGame not the same after two joins")
	return false
  } else if joinedGame.State != 2 {
    fmt.Println("unexpected state %d, expected %d", joinedGame.State, game.STARTED)
	return false
  } else if err != nil {
    fmt.Println("joinGame failed: ", err)
	return false
  } else {
    fmt.Printf("joinGame successful, game = %+v\n\n", joinedGame)
  }
  
  game.QuestionSelect(createdGame.GameId, createdGame.Categories[1], 30)

  full, err := game.RegisterBuzz(createdGame.GameId, playerId0, 4328912, false)
  fmt.Println(full)
  full, err = game.RegisterBuzz(createdGame.GameId, playerId1, 8293, false)
  fmt.Println(full)
  full, err = game.RegisterBuzz(createdGame.GameId, playerId2, 3459032, false)
  fmt.Println(full)

  answered, id, err := game.GetNewCurrentPlayer(createdGame.GameId)
  fmt.Println(answered)
  fmt.Printf("new current player: %s\n", id)

  a, b, c, d := game.IncomingAnswer(createdGame.GameId, playerId1, 2)
  fmt.Println(a)
  fmt.Println(b)
  fmt.Printf("%+v\n", c)
  fmt.Println(d)
  
  gbytes, err := json.Marshal(createdGame)
  if err != nil {
    fmt.Println("json.Marshal failed: ", err)
	return false
  }
  
  //fmt.Println(gbytes)
  testMsg := struct {
    GameId string `json:"gameId"`
	State int `json:"state"`
	Players []game.Player `json:"players"`
	Categories []string `json:"categories"`
	RemainingQuestions uint8 `json:"remainingQuestions"`
  }{}
  err = json.Unmarshal(gbytes, &testMsg)
  if err != nil {
    fmt.Println("json.Unmarshal failed: ", err)
	return false
  }
  
  fmt.Printf("marshaled and unmarshaled game = %+v\n\n", testMsg)
  //fmt.Println("%d", err)
  fmt.Println("TEST DONE")

  return true
  
}

func testPrintCategories() bool {
  questions.PopulateCategories()
  return true
}

func main() {
  //fmt.Println("goodbye, world")
  
  //questions.PopulateCategories()
  
  success := testCreateJoinJoin()
  if !success {
    fmt.Println("testCreateJoinJoin failed")
	return
  }
  
  success = testPrintCategories()
  if !success {
    fmt.Println("testPrintCategories failed")
	return
  }
  
  return
}