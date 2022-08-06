package game

import (
  "errors"
  "fmt"
  "gogo-sockets/game/questions"
  "github.com/google/uuid"
  cmap "github.com/orcaman/concurrent-map"
  "math/rand"
)

// games "database"
var gMap = cmap.New()

func AllGames() ([]*Game, error) {
  // we need to copy all the games to a slice
  itms := gMap.Items()

  gls := make([]*Game, 0)

  for _, v  := range itms {
    if g, ok := v.(*Game); ok {
      gls = append(gls, g)
      continue
    }

    // found a non-game value, ack!
    return nil, errors.New("Found a non-game value in the games database")
  }

  return gls, nil;
}

func GetGame(gameId string) (*Game, bool) {
  iface, ok := gMap.Get(gameId)
  if !ok {
    return nil, ok
  }

  g, ok := iface.(*Game)
  if !ok {
    return nil, ok
  }

  return g, true
}

func SetGameState(gameId string, state GameState) *Game {
  v := gMap.Upsert(
    gameId, 
    nil, 
    func(exist bool, valInMap, newVal interface{}) interface{} {
      xg, ok := (valInMap).(*Game)
      if !ok {
        panic("Setting game state on a game not in map")
      }

      xg.State = state
      return xg
    })

  g, ok := (v).(*Game)
  if !ok {
    panic("Setting game state on a game not in map")
  }

  return g
}

// a player just disconnected, we need to remove them from the game
func RemovePlayer(playerId string) (*Game, bool) {
	
	itms := gMap.Items()

	for _, v  := range itms {
      if g, ok := v.(*Game); ok {
	    for i := 0; i < len(g.Players); i++ {
	      if g.Players[i].PlayerId == playerId {
			newPlayers := g.Players[:i]
			newPlayers = append(newPlayers, g.Players[i+1:]...)
			g.Players = newPlayers
			break
	      }
	    }
	  
	    if len(g.Players) == 0 {
	      // delete game
		  return g, true
	    } else {
	      // for now just do the same thing
		  // might change this later
	      return g, false
	    }
	  }
    }
  
    return nil, false
  
}

func RemoveGame(gameId string) {
	
	gMap.Remove(gameId)

}

func CreateGame(host, hostname string, numCategories, questionsPerCategory, totalQuestions uint8) *Game {
  gameId := uuid.NewString()
  
  // define the host player
  hostPlayer := &Player{
    PlayerId: host,
	Name: hostname,
    Score: 0,
	CurrentPlayer: true,
  }
  
  // make sure the categories have been loaded
  if !questions.CategoriesInitialized {
    questions.PopulateCategories()
  }

  // remaining questions should be the requested total questions
  // unless the requested total questions is more than the total 
  // number of questions.
  if (numCategories * questionsPerCategory) < totalQuestions {
    totalQuestions = (numCategories * questionsPerCategory)
  }
  
  game := &Game{
    GameId: gameId,
    Players: []*Player{ hostPlayer },
    Categories: questions.GetGameCategories(gameId, numCategories, questionsPerCategory),
	RemainingQuestions: totalQuestions,
    CurrentPlayerId: host,
  }

  gMap.Set(gameId, game)

  return game
}

func JoinGame(gameId, playerId, playerName string) (*Game, error) {
  g, ok := GetGame(gameId)
  if !ok {
    return nil, fmt.Errorf("In Join, Unknown game: %q", gameId)
  }

  // define the new player
  newPlayer := &Player{
    PlayerId: playerId,
	Name: playerName,
	Score: 0,
	CurrentPlayer: false,
  }

  if g.State != WAITING {
    return nil, errors.New("Game not waiting for players")
  }

  if len(g.Players) > 2 {
    return nil, errors.New("Game already has more than 2 players")
  }

  g.Players = append(g.Players, newPlayer)

  if len(g.Players) == 3 {
    g.State = SPIN
  }

  gMap.Set(g.GameId, g)
  
  return g, nil
}

// Removes player from game. If the player is the only player in the
// game the game is removed. 
func LeaveGame(gameId, player string) (*Game, error) {
  g, ok := GetGame(gameId)
  if !ok {
    return nil, fmt.Errorf("In Leave, Unknown game: %q", gameId)
  }


  newPlayers := make([]*Player, 0)
  for _, p := range g.Players {
    if player != p.PlayerId {
      newPlayers = append(newPlayers, p)
    }
  }


  if len(newPlayers) == 0 { 
    gMap.Remove(gameId)
    return nil, nil
  }
  
  if g.CurrentPlayerId == player {
    g.CurrentPlayerId = g.Players[0].PlayerId
  }

  g.State = WAITING

  // otherwise update the game
  g.Players = newPlayers
  gMap.Set(gameId, g)

  return g, nil
}

func UpdateQuestionCount(gameId string, qcount uint8) (*Game, error) {

  g, ok := GetGame(gameId)
  if !ok {
    return nil, fmt.Errorf("In UpdateQuestionCount, Unknown game: %q", gameId)
  }

  g.State = SPIN
  g.RemainingQuestions = qcount

  // otherwise update the game
  gMap.Set(gameId, g)

  return g, nil
}

func QuestionSelect(gameId, category string, pointValue uint8) (Question, error) {
	g, ok := GetGame(gameId)
	if !ok {
		return Question{}, fmt.Errorf("In QuestionSelect, Unknown game: %q", gameId)
	}
	
	qInternal := questions.GetGameQuestion(gameId, category, pointValue)
	
	p := rand.Perm(4)
	choicesList := []string{"", "", "", ""}
	
	choicesList[p[0]] = qInternal.Correct
	choicesList[p[1]] = qInternal.Incorrect[0]
	choicesList[p[2]] = qInternal.Incorrect[1]
	choicesList[p[3]] = qInternal.Incorrect[2]
	
	qSend := Question{
		Category: category,
		PointValue: pointValue,
		Text: qInternal.QuestionText,
		Choices: choicesList,
		correctIndex: uint8(p[0]),
		buzzes: []*Buzz{},
	}
	
	g.currentQuestion = &qSend
  g.State = QUESTION
	
	return qSend, nil
}


func RegisterBuzz(gameId, clientId string, delay uint32, expired bool) (bool) {
  // type UpsertCb func(exist bool, valueInMap interface{}, newValue interface{}) interface{}
  // Upsert(key string, value interface{}, cb UpsertCb) (res interface{}) 
  v := gMap.Upsert(gameId, nil, func(exist bool, valInMap interface{}, newVal interface{}) interface{} {
      if (!exist) {
        panic("Registering a buzz on a non-existent game")
      }

      existingGame, ok := valInMap.(*Game)
      if !ok {
        panic("Registering a buzz on an invalid game in map")
      }

      if existingGame.currentQuestion == nil {
        panic("Registering a buzz on a nil currentQuestion")
      }

	    if len(existingGame.currentQuestion.buzzes) == 3 {
        //panic("Registering a buzz on a game with 3 buzzes")
        fmt.Println("registering a buzz on a game with 3 buzzes, clientId: ", clientId)
        return existingGame
      }

      buzz := Buzz{
        playerId: clientId,
        delay: delay,
        expired: expired,
      }

      existingGame.currentQuestion.buzzes = append(
        existingGame.currentQuestion.buzzes, &buzz)

      return existingGame
  })
  
  g, ok := (v).(*Game)
  if !ok {
    panic("Got something other than a *Game from map upsert")
  }

	if len(g.currentQuestion.buzzes) == 3 {
		return true 
	} else {
		return false  
	}
}

func SetNewCurrentPlayer(g *Game) (bool, *Game, error) {

  expired := true
	
  v := gMap.Upsert(g.GameId, g, func(exist bool, valInMap, newVal interface{}) interface{} {
    bestTime := uint32(1 << 16)
    existingGame, ok := (valInMap).(*Game)
    if !ok {
      panic("Non-*Game in map")
    }

    // default to the existing current player
    winner := existingGame.CurrentPlayerId

    if existingGame.currentQuestion == nil {
      panic("Setting current player with a nil currentQuestion")
    }

    for _, b := range existingGame.currentQuestion.buzzes {
      if b.delay < bestTime  {
        bestTime = b.delay
        winner = b.playerId
        expired = false
      }
    }

    if bestTime == 1 << 16 {
      // no change, current player is current player
      return existingGame
    }

    existingGame.SetCurrentPlayer(winner)

    return existingGame
  })

  g, ok := (v).(*Game)
  if !ok {
    panic("Non-*Game retured from set currentplayer upsert")
  }

  return expired, g, nil
}

func IncomingAnswer(gameId, clientId string, answerIndex uint8) (bool, int, *Game, error) {
	g, ok := GetGame(gameId)
	if !ok {
		return false, -1, &Game{}, fmt.Errorf("In IncomingAnswer, Unknown game: %q", gameId)
	}

	correct := false
	player := g.GetPlayerByUuid(clientId)
	if player != nil {
		correct = g.currentQuestion.correctIndex == answerIndex
		player.updateScore(g.currentQuestion.PointValue, correct)
	} // else (actually not an error)
	  // client.go calls IncomingAnswer with a blank clientId if the question expires

    // save the correctIndex to return
	correctIndex := g.currentQuestion.correctIndex
	
	// we are done with this question
	questions.RemoveGameQuestion(gameId, g.currentQuestion.Category, g.currentQuestion.PointValue)
	g.currentQuestion = nil
	g.RemainingQuestions -= 1
	
	if g.RemainingQuestions == 0 {
		g.State = ENDED
	}
	
	return correct, int(correctIndex), g, nil
}






