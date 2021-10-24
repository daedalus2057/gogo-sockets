// Stollen liberally from the chat example in
// gorilla/websocket -- old code, from 2014 ish.
// Author -- them and me, donovan nye

package main

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[string]*Client

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[string]*Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client.ClientId] = client
		case client := <-h.unregister:
			if _, ok := h.clients[client.ClientId]; ok {
				delete(h.clients, client.ClientId)
				close(client.Send)
			}
		case message := <-h.broadcast:
			for clientId, client := range h.clients {
				select {
				case client.Send <- message:
				default:
					close(client.Send)
					delete(h.clients, clientId)
				}
			}
		}
	}
}
