package game


type GameState int
const (
  WAITING GameState = iota
  UNKNOWN
  START_ROUND
)

type Player struct {
  PlayerId string `json:"playerId"`
  // highest/lowest score a player can have if 6 categories is 6*(10*(5+4+...+1)) = +/-900 -> int16
  Score int16 `json:"score"` 
  //host bool		// is this the host player? doesn't export to json
}

type Game struct {
  GameId string `json:"gameId"`
  State GameState `json:"gameState"`
  // by convention, first player is host
  Players []Player
  Categories []string
}



