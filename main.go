// Stollen liberally from the chat example in 
// gorilla/websocket -- old code, from 2014 ish.
// Author -- them and me, donovan nye

package main

import (
  "fmt"
  "flag"
  "log"
  "net/http"
)


var addr = flag.String("addr", ":8080", "http service address")

func main() {
  // get the addr
  flag.Parse()

  // the brodcaster, writer, reader extrodinair
  // thank you gorilla!!
  hub := newHub()
  // run in it's own goroutine
  go hub.run()

  // basic handlers from golang
  // to be upgraded
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    serveWs(hub, w, r)
  })

  // http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
  //   fmt.Fprintf(w,"Listing on port %v", addr)
  // })

  fmt.Println("Listenting on ", *addr)
  err := http.ListenAndServe(*addr, nil)
  if err != nil {
    log.Fatal("ListenAndServe failed:", err)
  }
}

