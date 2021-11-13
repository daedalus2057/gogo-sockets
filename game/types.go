package game

type GameState int
const (
  WAITING GameState = iota
  UNKNOWN
  STARTED
  ENDED
  SPIN
)

type Player struct {
  PlayerId string `json:"playerId"`
  
  // TODO: send me the name
  Name string `json:"name"`
  
  // highest/lowest score a player can have if 6 categories is 6*(10*(5+4+...+1)) = +/-900 -> int16
  Score int16 `json:"score"` 
  CurrentPlayer bool // is this player is the current player?
  //host bool		// is this the host player? doesn't export to json
}

func (p *Player) updateScore(pointValue uint8, correct bool) {
  if correct {
    p.Score += int16(pointValue)
  } else {
    p.Score -= int16(pointValue)
  }
}

type Buzz struct {
  playerId string
  delay uint32 // TODO: milliseconds? microseconds?
  expired bool // did the player actually buzz or did time expire?
}

type Question struct {
  Category string `json:"category"`
  PointValue uint8 `json:"pointValue"`
  Text string `json:"text"`
  Choices []string `json:"choices"`
  correctIndex uint8 
  buzzes []*Buzz
}

type Game struct {
  GameId string `json:"gameId"`
  State GameState `json:"gameState"`
  // by convention, first player is host
  Players []*Player `json:"players"`
  Categories []string `json:"categories"`
  RemainingQuestions uint8 `json:"remainingQuestions"`
  CurrentPlayerId string `json:"currentPlayerId"`
  
  // non-exported
  currentQuestion *Question
  
}

func (g *Game) GetPlayerByUuid(playerId string) *Player {
  for _, player := range g.Players {
    if player.PlayerId == playerId {
      return player
    }
  }
  return nil
}

func (g *Game) SetCurrentPlayer(playerId string) {
  g.CurrentPlayerId = playerId

  for _, player := range g.Players {
    if player.PlayerId == playerId {
	  player.CurrentPlayer = true
	} else {
	  player.CurrentPlayer = false
	}
  }
}
