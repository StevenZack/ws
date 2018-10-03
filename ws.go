package ws

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
	"time"
)

var (
	TextMessage = websocket.TextMessage
)

type Server struct {
	HttpServer  *http.Server
	preHandlers []func(ResponseWriter, *Request)
	r, mr       map[string]func(ResponseWriter, *Request)
	upgrader    websocket.Upgrader
}
type Request struct {
	RequestURL string
	Body       []byte
	Headers    map[string]string
}
type ResponseWriter interface {
	WriteMessage(int, []byte) error
	Close() error
}

func (r *Request) ScanBody(i interface{}) error {
	if r.Body == nil {
		return errors.New("body is nil")
	}
	return json.Unmarshal(r.Body, i)
}
func NewServer(addr string) *Server {
	s := &Server{}
	s.HttpServer = &http.Server{Addr: addr, Handler: s}
	s.r = make(map[string]func(ResponseWriter, *Request))
	s.mr = make(map[string]func(ResponseWriter, *Request))
	return s
}
func (s *Server) ListenAndServe() error {
	return s.HttpServer.ListenAndServe()
}
func (s *Server) Stop() error {
	if s != nil {
		ctx, canel := context.WithTimeout(context.Background(), time.Second)
		defer canel()
		e := s.HttpServer.Shutdown(ctx)
		return e
	}
	return nil
}
func (s *Server) HandleFunc(url string, f func(ResponseWriter, *Request)) {
	s.r[url] = f
}

func (s *Server) HandleMultiReqs(url string, f func(ResponseWriter, *Request)) {
	s.mr[url] = f
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c, e := s.upgrader.Upgrade(w, r, nil)
	if e != nil {
		fmt.Println("Upgrade() error:", e)
		return
	}
	for {
		_, data, e := c.ReadMessage()
		if e != nil {
			return
		}
		go handleMsg(s, c, data)
	}
}
func handleMsg(s *Server, c ResponseWriter, data []byte) {
	reader := bufio.NewReader(bytes.NewReader(data))
	burl, _, e := reader.ReadLine()
	if e != nil {
		fmt.Println("readURLLine() error:", e)
		return
	}
	body, _, e := reader.ReadLine()
	if e != nil {
		fmt.Println("readBodyLine() error:", e)
		return
	}
	r := &Request{RequestURL: string(burl), Body: body}
	r.Headers = make(map[string]string)
	for _, v := range s.preHandlers {
		v(c, r)
	}
	url := strings.Split(r.RequestURL, "?")[0]
	if h, ok := s.r[url]; ok {
		h(c, r)
	} else if k, ok := hasPreffixInMap(s.mr, r.RequestURL); ok {
		s.mr[k](c, r)
	} else {
		c.WriteMessage(websocket.TextMessage, []byte(`{"Status":"ERR","Info":"404 not found"}`))
	}
	c.Close()
}

func hasPreffixInMap(m map[string]func(ResponseWriter, *Request), p string) (string, bool) {
	for k, _ := range m {
		if len(p) >= len(k) && k == p[:len(k)] {
			return k, true
		}
	}
	return "", false
}
func (s *Server) AddPrehandler(f func(ResponseWriter, *Request)) {
	s.preHandlers = append(s.preHandlers, f)
}
