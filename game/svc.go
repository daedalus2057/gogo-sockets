package game

import (
  "errors"
  "fmt"
  "gogo-sockets/game/questions"
  "github.com/google/uuid"
  cmap "github.com/orcaman/concurrent-map"
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

func CreateGame(host string) *Game {
  gameId := uuid.NewString()
  
  // define the host player
  hostPlayer := Player{
    PlayerId: host,
    Score: 0,
  }
  
  // make sure the categories have been loaded
  if !questions.CategoriesInitialized {
    questions.PopulateCategories()
  }
  
  game := &Game{
    GameId: gameId,
    Players: []Player{ hostPlayer },
    Categories: questions.GetGameCategories(),
  }

  gMap.Set(gameId, game)

  return game
}

func JoinGame(gameId, player string) (*Game, error) {
  g, ok := GetGame(gameId)
  if !ok {
    return nil, fmt.Errorf("Unknown game: %q", gameId)
  }

  // define the new player
  newPlayer := Player{
    PlayerId: player,
	Score: 0,
  }

  if g.State != WAITING {
    return nil, errors.New("Game not waiting for players")
  }

  if len(g.Players) > 2 {
    return nil, errors.New("Game already has more than 2 players")
  }

  g.Players = append(g.Players, newPlayer)

  if len(g.Players) == 3 {
    g.State = START_ROUND
  }

  gMap.Set(g.GameId, g)
  
  return g, nil
}
