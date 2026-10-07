package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type SSEMessage struct {
	Event     string `json:"event"`
	Data      string `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type SSEBroker struct {
	mu      sync.RWMutex
	clients map[chan SSEMessage]bool
}

var GlobalSSEBroker = &SSEBroker{
	clients: make(map[chan SSEMessage]bool),
}

func (b *SSEBroker) Register() chan SSEMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan SSEMessage, 16)
	b.clients[ch] = true
	return ch
}

func (b *SSEBroker) Unregister(ch chan SSEMessage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
}

func (b *SSEBroker) Broadcast(event, data string) {
	msg := SSEMessage{
		Event:     event,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func SSEHandler(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	clientChan := GlobalSSEBroker.Register()
	defer GlobalSSEBroker.Unregister(clientChan)

	initMsg := SSEMessage{
		Event:     "CONNECTED",
		Data:      "SSE notification stream connected",
		Timestamp: time.Now().Unix(),
	}
	initBytes, _ := json.Marshal(initMsg)
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", initMsg.Event, string(initBytes))
	c.Writer.Flush()

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			jsonBytes, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", msg.Event, string(jsonBytes))
			c.Writer.Flush()
		}
	}
}
