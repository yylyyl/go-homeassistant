package homeassistant

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"sync"
	"time"
)

type WebsocketConfig struct {
	// Example: ws://homeassistant.local:8123/api/websocket
	Url string
	// API key
	Authorization string
	// Optional, a default dialer will be used if omitted
	WebSocketDialer *websocket.Dialer

	debug bool
}

type eventType uint

const (
	eventTypeState eventType = iota
	eventTypeTrigger
)

type eventReceiver struct {
	eventType eventType
	stateCh   chan<- ChangedState
	triggerCh chan<- []byte
}

type WebsocketClient struct {
	url           string
	authorization string

	debug    bool
	wsDialer *websocket.Dialer
	wsConn   *websocket.Conn
	wsMsgCh  chan *wsMsg
	wsMutex  sync.RWMutex
	reqId    uint
	reqMap   map[uint]*wsReqItem

	eventMutex sync.Mutex
	eventMap   map[uint]*eventReceiver

	closed  bool
	closeCh []chan struct{}
}

type wsMsg struct {
	msg []byte
	err error
}

// DialWebsocket connects to a home-assistant instance using websocket
func DialWebsocket(ctx context.Context, cfg *WebsocketConfig) (*WebsocketClient, error) {
	if cfg == nil {
		panic("empty home-assistant config")
	}

	c := &WebsocketClient{
		url:           cfg.Url,
		authorization: cfg.Authorization,
		wsDialer:      cfg.WebSocketDialer,
		debug:         cfg.debug,

		wsMsgCh:  make(chan *wsMsg),
		reqMap:   map[uint]*wsReqItem{},
		eventMap: map[uint]*eventReceiver{},
	}
	if c.wsDialer == nil {
		c.wsDialer = websocket.DefaultDialer
	}

	if err := c.dial(ctx); err != nil {
		return nil, fmt.Errorf("failed to dial websocket API: %v", err)
	}

	return c, nil
}

func (c *WebsocketClient) dial(ctx context.Context) error {
	conn, _, err := c.wsDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %v", err)
	}

	c.wsConn = conn
	go c.wsReceiver()

	err = c.handshake(ctx)
	if err != nil {
		return fmt.Errorf("failed to handshake: %v", err)
	}

	go c.wsHandler()

	return nil
}

func (c *WebsocketClient) handshake(ctx context.Context) error {
	buf, err := c.getHandshakeMsgWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to read ws 1: %v", err)
	}

	var resp wsMsgAuthChallengeOrResult
	err = json.Unmarshal(buf, &resp)
	if err != nil {
		return fmt.Errorf("failed to decode json 1: %v", err)
	}

	if resp.Type != "auth_required" {
		return fmt.Errorf("unexpected msg: %s %s", resp.Type, resp.HAVersion)
	}

	err = c.wsConn.WriteJSON(&wsMsgAuthToken{
		Type:        "auth",
		AccessToken: c.authorization,
	})
	if err != nil {
		return fmt.Errorf("failed to send ws: %v", err)
	}

	buf, err = c.getHandshakeMsgWithContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to read ws 2: %v", err)
	}

	err = json.Unmarshal(buf, &resp)
	if err != nil {
		return fmt.Errorf("failed to decode json 2: %v", err)
	}

	if resp.Type != "auth_ok" {
		return fmt.Errorf("unexpected result: %s %s %s", resp.Type, resp.HAVersion, resp.Message)
	}

	if c.debug {
		log.Printf("ha version: %s", resp.HAVersion)
	}

	return nil
}

// wsReceiver receives messages from websocket, and then pass it to getHandshakeMsgWithContext or wsHandler
func (c *WebsocketClient) wsReceiver() {
	defer c.wsConn.Close()
	defer close(c.wsMsgCh)

	for {
		_, buf, err := c.wsConn.ReadMessage()
		if err != nil {
			if c.debug {
				log.Printf("ws receiver read err: %v", err)
			}
			c.wsMsgCh <- &wsMsg{err: fmt.Errorf("failed to read ws: %v", err)}
			break
		}

		if c.debug {
			log.Printf("ws read: %s", buf)
		}
		c.wsMsgCh <- &wsMsg{msg: buf}
	}

	// clear up
	if c.debug {
		log.Printf("ws receiver closed")
	}

	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()

	for id, req := range c.reqMap {
		select {
		case req.Ch <- &wsResp{Err: fmt.Errorf("ws closed")}:
		default:
			// you don't care
		}
		close(req.Ch)
		delete(c.reqMap, id)
	}

	for id, receiver := range c.eventMap {
		switch receiver.eventType {
		case eventTypeState:
			close(receiver.stateCh)
		case eventTypeTrigger:
			close(receiver.triggerCh)
		default:
			log.Printf("WARN unimplemented event: %d", receiver.eventType)
		}
		delete(c.eventMap, id)
	}

	c.closed = true
	for _, closeCh := range c.closeCh {
		closeCh <- struct{}{}
	}
}

// Closed returns a channel for the close of websocket connection
func (c *WebsocketClient) Closed() <-chan struct{} {
	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()

	ch := make(chan struct{})
	if c.closed {
		go func() {
			ch <- struct{}{}
		}()
		return ch
	}

	c.closeCh = append(c.closeCh, ch)
	return ch
}

// Close closes the websocket connection
func (c *WebsocketClient) Close() error {
	return c.wsConn.Close()
}

func (c *WebsocketClient) getHandshakeMsgWithContext(ctx context.Context) ([]byte, error) {
	select {
	case m := <-c.wsMsgCh:
		return m.msg, m.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type wsMsgAuthChallengeOrResult struct {
	Type      string `json:"type"`
	HAVersion string `json:"ha_version"`
	Message   string `json:"message"`
}

type wsMsgAuthToken struct {
	Type        string `json:"type"`
	AccessToken string `json:"access_token"`
}

type wsMsgCommonReq struct {
	ID   uint   `json:"id"`
	Type string `json:"type"`

	// subscribe_events
	EventType string `json:"event_type,omitempty"`
	// subscribe_trigger
	Trigger any `json:"trigger,omitempty"`
	// unsubscribe_events
	Subscription uint `json:"subscription,omitempty"`

	// call_service
	Domain string `json:"domain,omitempty"`
	// call_service
	Service string `json:"service,omitempty"`
	// call_service
	ServiceData map[string]any `json:"service_data,omitempty"`
	// call_service
	Target *wsMsgCommonReqTarget `json:"target,omitempty"`
}

type wsMsgCommonReqTarget struct {
	EntityId string `json:"entity_id"`
}

type wsMsgCommonResp struct {
	ID      uint            `json:"id"`
	Type    string          `json:"type"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result"`
	Error   struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Event struct {
		// possible value: state_changed or nil
		EventType string `json:"event_type"`
		// state event
		Data ChangedState `json:"data"`
		// trigger
		Variables json.RawMessage `json:"variables"`
		// state event
		Origin string `json:"origin"`
		// state event
		TimeFired time.Time `json:"time_fired"`
		// common field
		Context any `json:"context"`
	} `json:"event"`
}

type wsResp struct {
	Err error
	Buf []byte
}

type wsReqItem struct {
	Ch chan *wsResp
}

// wsHandler dispatches incoming messages
func (c *WebsocketClient) wsHandler() {
	for m := range c.wsMsgCh {
		if m.err != nil {
			log.Printf("ERR %v", m.err)
			break
		}

		var resp wsMsgCommonResp
		err := json.Unmarshal(m.msg, &resp)
		if err != nil {
			log.Printf("failed to decode json: %v", err)
			continue
		}

		if resp.Type == "event" {
			c.eventMutex.Lock()

			receiver := c.eventMap[resp.ID]
			if receiver == nil {
				log.Printf("WARN wild event: %v", resp)
				c.eventMutex.Unlock()
				continue
			}

			switch receiver.eventType {
			case eventTypeState:
				receiver.stateCh <- resp.Event.Data
			case eventTypeTrigger:
				receiver.triggerCh <- resp.Event.Variables
			default:
				log.Printf("WARN unimplemented event: %d", receiver.eventType)
			}

			c.eventMutex.Unlock()
			continue
		}

		if resp.Type != "result" && resp.Type != "pong" {
			log.Printf("WARN unrecognized type: %s", resp.Type)
			continue
		}

		c.wsMutex.Lock()

		req := c.reqMap[resp.ID]
		if req == nil {
			c.wsMutex.Unlock()
			log.Printf("request not found: %d", resp.ID)
			continue
		}

		select {
		case req.Ch <- &wsResp{Buf: m.msg}:
		default:
			// timeout, the response is discarded
		}

		close(req.Ch)
		delete(c.reqMap, resp.ID)
		c.wsMutex.Unlock()
	}

	if c.debug {
		log.Printf("ws handler closed")
	}
}

func (c *WebsocketClient) wsSend(req wsMsgCommonReq, respCh chan *wsResp) {
	c.wsMutex.Lock()
	defer c.wsMutex.Unlock()

	c.reqId++
	req.ID = c.reqId

	c.reqMap[req.ID] = &wsReqItem{
		Ch: respCh,
	}

	if c.debug {
		s, _ := json.Marshal(&req)
		log.Printf("ws write: %s", s)
	}

	// mutex protected, do not write multiple messages at the same time
	err := c.wsConn.WriteJSON(&req)
	if err != nil {
		respCh <- &wsResp{Err: fmt.Errorf("failed to send ws: %v", err)}
		close(respCh)
		delete(c.reqMap, req.ID)
	}
}

func (c *WebsocketClient) wsCommonCall(ctx context.Context, req wsMsgCommonReq) (*wsMsgCommonResp, error) {
	respCh := make(chan *wsResp)
	c.wsSend(req, respCh)

	var resp *wsResp

	select {
	case <-ctx.Done():
		// timeout, the response is discarded
		if c.debug {
			log.Printf("response dropped: %d %s", req.ID, req.Type)
		}
		return nil, ctx.Err()
	case resp = <-respCh:
	}

	if resp.Err != nil {
		return nil, resp.Err
	}

	var r wsMsgCommonResp
	err := json.Unmarshal(resp.Buf, &r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ws response: %v", err)
	}

	return &r, nil
}

func wsGetResult(input *wsMsgCommonResp) ([]byte, error) {
	if !input.Success {
		return nil, fmt.Errorf("unexpected failed resp: %s, %s", input.Error.Code, input.Error.Message)
	}
	return input.Result, nil
}
