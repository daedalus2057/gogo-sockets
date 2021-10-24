package game


type GameState int
const (
  WAITING GameState = iota
  UNKNOWN
  START_ROUND
)


type Game struct {
  GameId string
  State GameState
  // by convention, first player is host
  Players []string
}



