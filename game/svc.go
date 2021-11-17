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

func RemoveGame(gameId string) {
	
	gMap.Remove(gameId)

}

func CreateGame(host string) *Game {
  gameId := uuid.NewString()
  
  // define the host player
  hostPlayer := &Player{
    PlayerId: host,
    Score: 0,
	CurrentPlayer: true,
  }
  
  // make sure the categories have been loaded
  if !questions.CategoriesInitialized {
    questions.PopulateCategories()
  }
  
  game := &Game{
    GameId: gameId,
    Players: []*Player{ hostPlayer },
    Categories: questions.GetGameCategories(gameId),
	  RemainingQuestions: 30,
    CurrentPlayerId: host,
  }

  gMap.Set(gameId, game)

  return game
}

func JoinGame(gameId, player string) (*Game, error) {
  g, ok := GetGame(gameId)
  if !ok {
    return nil, fmt.Errorf("In Join, Unknown game: %q", gameId)
  }

  // define the new player
  newPlayer := &Player{
    PlayerId: player,
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
    g.State = STARTED
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
	
	return qSend, nil
}


func RegisterBuzz(gameId, clientId string, delay uint32, expired bool) (bool, error) {
	g, ok := GetGame(gameId)
	if !ok {
		return false, fmt.Errorf("In RegisterBuzz, Unknown game: %q", gameId)
	}
	
	buzz := Buzz{
		playerId: clientId,
		delay: delay,
		expired: expired,
	}
	
	g.currentQuestion.buzzes = append(g.currentQuestion.buzzes, &buzz)

	if len(g.currentQuestion.buzzes) == 3 {
		return true, nil
	} else {
		return false, nil
	}
}

func SetNewCurrentPlayer(g *Game) (bool, string, error) {
	
	minDelay := uint32(0xFFFFFFFF)
	retId := ""
	for _, buzz := range(g.currentQuestion.buzzes) {
		if !buzz.expired && buzz.delay < minDelay {
			minDelay = buzz.delay
			retId = buzz.playerId
		}
	}
	
	if retId != "" {
		g.SetCurrentPlayer(retId)
		return false, retId, nil
	} else {
		return true, retId, nil
	}
}

func IncomingAnswer(gameId, clientId string, answerIndex uint8) (bool, int, *Game, error) {
	g, ok := GetGame(gameId)
	if !ok {
		return false, -1, &Game{}, fmt.Errorf("In IncomingAnswer, Unknown game: %q", gameId)
	}

	correct := false
	player := g.GetPlayerByUuid(clientId)
	if player != nil {
    fmt.Printf("WTF %v=%v\n", g.currentQuestion.correctIndex, answerIndex)
		correct := g.currentQuestion.correctIndex == answerIndex
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






