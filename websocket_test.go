package homeassistant

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testWs struct {
	Server    *httptest.Server
	URL       string
	TestFunc1 func(m map[string]any) (interface{}, error)
	TestRead2 bool
	TestFunc2 func(m map[string]any) interface{}
	TestRead3 bool
	TestFunc3 func(m map[string]any) interface{}
}

func (s *testWs) handler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - Cannot upgrade to websocket"))
		return
	}
	defer c.Close()

	var m map[string]any
	c.WriteJSON(map[string]any{"type": "auth_required", "ha_version": "2021.5.3"})
	// ignore authentication
	c.ReadJSON(&m)
	c.WriteJSON(map[string]any{"type": "auth_ok", "ha_version": "2021.5.3"})

	c.ReadJSON(&m)

	ret, err := s.TestFunc1(m)
	if ret == nil {
		e := map[string]any{"code": "error", "message": err.Error()}
		c.WriteJSON(map[string]any{"id": m["id"], "type": "result", "success": false, "error": e})
		return
	}

	c.WriteJSON(ret)

	if s.TestFunc2 != nil {
		if s.TestRead2 {
			c.ReadJSON(&m)
		}
		c.WriteJSON(s.TestFunc2(m))
	}
	if s.TestFunc3 != nil {
		if s.TestRead3 {
			c.ReadJSON(&m)
		}
		c.WriteJSON(s.TestFunc3(m))
	}

	// do not close the websocket connection
	time.Sleep(time.Second)
}

func (s *testWs) Run() {
	s.Server = httptest.NewServer(http.HandlerFunc(s.handler))
	s.URL = strings.Replace(s.Server.URL, "http", "ws", -1)
}
func (s *testWs) Close() {
	s.Server.Close()
}

func TestWebsocketClient_Ping(t *testing.T) {
	ts := &testWs{TestFunc1: func(m map[string]any) (interface{}, error) {
		return map[string]any{"id": m["id"], "type": "pong"}, nil
	}}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = ha.Ping(ctx)
	if err != nil {
		t.Fatalf("failed to ping: %v", err)
	}
}

func TestWebsocketClient_Ping_Timeout(t *testing.T) {
	ts := &testWs{
		TestFunc1: func(m map[string]any) (interface{}, error) {
			time.Sleep(time.Second * 2)
			return map[string]any{"id": m["id"], "type": "pong"}, nil
		},
		TestRead2: true,
		TestFunc2: func(m map[string]any) interface{} {
			return map[string]any{"id": m["id"], "type": "pong"}
		},
	}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()

	err = ha.Ping(ctx2)
	if err == nil {
		t.Fatalf("ping should fail")
	}

	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel3()

	err = ha.Ping(ctx3)
	if err != nil {
		t.Fatalf("failed to ping: %v", err)
	}
}

func TestWebsocketClient_GetServices(t *testing.T) {
	ts := &testWs{TestFunc1: func(m map[string]any) (interface{}, error) {
		return map[string]any{
			"id":      m["id"],
			"type":    "result",
			"success": true,
			"result": map[string]any{
				"homeassistant": map[string]any{
					"turn_on": map[string]any{
						"name":        "nnnnn",
						"description": "ddd",
						"fields":      map[string]any{},
						"target": map[string]any{
							"entity": map[string]any{},
						},
					},
				},
			},
		}, nil
	}}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ret, err := ha.GetServices(ctx)
	if err != nil {
		t.Fatalf("failed to get services: %v", err)
	}

	if ret["homeassistant"]["turn_on"].Name != "nnnnn" {
		t.Fatalf("unexpected response: %v", ret)
	}
}

func TestWebsocketClient_GetStates(t *testing.T) {
	ts := &testWs{TestFunc1: func(m map[string]any) (interface{}, error) {
		return map[string]any{
			"id":      m["id"],
			"type":    "result",
			"success": true,
			"result": []map[string]any{
				{
					"entity_id": "light.one",
					"state":     "on",
				},
				{
					"entity_id": "switch.one",
					"state":     "off",
				},
			},
		}, nil
	}}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ret, err := ha.GetStates(ctx)
	if err != nil {
		t.Fatalf("failed to get states: %v", err)
	}

	if ret[0].EntityId != "light.one" || ret[0].State != "on" {
		t.Fatalf("unexpected response: %v", ret)
	}
	if ret[1].EntityId != "switch.one" || ret[1].State != "off" {
		t.Fatalf("unexpected response: %v", ret)
	}
}

func TestWebsocketClient_SubscribeEventsStateChanged(t *testing.T) {
	ts := &testWs{
		// subscribe
		TestFunc1: func(m map[string]any) (interface{}, error) {
			return map[string]any{
				"id":      m["id"],
				"type":    "result",
				"success": true,
				"result":  nil,
			}, nil
		},
		// event
		TestFunc2: func(m map[string]any) interface{} {
			return map[string]any{
				"id":   m["id"],
				"type": "event",
				"event": map[string]any{
					"data": map[string]any{
						"entity_id": "light.one",
						"new_state": map[string]any{
							"entity_id": "light.one",
							"state":     "on",
						},
						"old_state": map[string]any{
							"entity_id": "light.one",
							"state":     "off",
						},
					},
				},
			}
		},
		// unsubscribe
		TestRead3: true,
		TestFunc3: func(m map[string]any) interface{} {
			return map[string]any{
				"id":      m["id"],
				"type":    "result",
				"success": true,
				"result":  nil,
			}
		},
	}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ch, err := ha.SubscribeEventsStateChanged(ctx)
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	s := <-ch
	if s.EntityId != "light.one" || s.OldState.State != "off" || s.NewState.State != "on" {
		t.Fatalf("unexpected response: %v", s)
	}

	err = ha.UnsubscribeEventsStateChanged(ctx)
	if err != nil {
		t.Fatalf("failed to unsubscribe: %v", err)
	}
}

func TestWebsocketClient_CallService(t *testing.T) {
	ts := &testWs{TestFunc1: func(m map[string]any) (interface{}, error) {
		return map[string]any{
			"id":      m["id"],
			"type":    "result",
			"success": true,
			"result": map[string]any{
				"id":        "326ef27d19415c60c492fe330945f954",
				"parent_id": nil,
				"user_id":   "31ddb597e03147118cf8d2f8fbea5553",
			},
		}, nil
	}}
	ts.Run()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	ha, err := DialWebsocket(ctx, &WebsocketConfig{
		Url:           ts.URL,
		Authorization: "",
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	err = ha.CallService(ctx, "light", "turn_on", map[string]any{
		"color_name": "beige",
		"brightness": "101",
	}, "light.kitchen")
	if err != nil {
		t.Fatalf("failed to call service: %v", err)
	}
}
