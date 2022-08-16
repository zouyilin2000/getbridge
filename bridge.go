package getbridge

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync/atomic"
)

func unwrap(e error) {
	if e != nil {
		fmt.Println(e)
		panic(e)
	}
}

type Client struct {
	HTTP_Client *http.Client
	ServerHost  string
	inChan      chan []byte
	waitingCnt  atomic.Int32
}

func NewClient(HTTP_Client *http.Client, ServerHost string) *Client {
	return &Client{
		HTTP_Client: HTTP_Client,
		ServerHost:  ServerHost,
		inChan:      make(chan []byte),
		waitingCnt:  atomic.Int32{},
	}
}

func (c *Client) Post(message []byte) {
	go func() {
		url := fmt.Sprintf("https://%v", c.ServerHost)
		request, err := http.NewRequest("GET", url, bytes.NewReader(message))
		unwrap(err)
		c.waitingCnt.Add(1)
		response, err := c.HTTP_Client.Do(request)
		c.waitingCnt.Add(-1)
		if !os.IsTimeout(err) {
			unwrap(err)
			message, err := io.ReadAll(response.Body)
			response.Body.Close()
			unwrap(err)
			c.inChan <- message
		}
		if c.waitingCnt.Load() == 0 {
			c.Post(make([]byte, 0))
		}
	}()
}

func (c *Client) Get() []byte {
	message := <-c.inChan
	return message
}

func (c *Client) GetNoWait() ([]byte, bool) {
	select {
	case message := <-c.inChan:
		return message, true
	default:
		return nil, false
	}
}

type Callback interface {
	HandleNewMessage(*Server, []byte)
	HandleNoMessage(*Server)
}

func newMessage(message []byte) bool {
	return len(message) > 0
}

type Server struct {
	Callback Callback
	outChan  chan []byte
}

func NewServer(Callback Callback) *Server {
	return &Server{
		Callback: Callback,
		outChan:  make(chan []byte),
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	message, err := io.ReadAll(r.Body)
	unwrap(err)

	if newMessage(message) {
		go s.Callback.HandleNewMessage(s, message)
	} else {
		go s.Callback.HandleNoMessage(s)
	}

	message = <-s.outChan
	w.Write(message) // drop if timeout
}

func (s *Server) Post(message []byte) {
	s.outChan <- message
}
