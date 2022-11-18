package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func (c *WebsocketClient) Ping(ctx context.Context) error {
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{Type: "ping"})
	if err != nil {
		return err
	}

	if resp.Type != "pong" {
		return fmt.Errorf("unecpected response of ping: %s", resp.Type)
	}

	return nil
}

type EntityList []Entity
type Entity struct {
	EntityId    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
	Context     any            `json:"context"`
}

func (c *WebsocketClient) GetStates(ctx context.Context) (EntityList, error) {
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{Type: "get_states"})
	if err != nil {
		return nil, err
	}

	raw, err := wsGetResult(resp)
	if err != nil {
		return nil, err
	}

	var ret EntityList
	err = json.Unmarshal(raw, &ret)

	return ret, nil
}

type ServiceList map[string]map[string]Service
type Service struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Fields      map[string]struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Required    *bool          `json:"required"`
		Example     any            `json:"example"`
		Selector    map[string]any `json:"selector"`
	} `json:"fields"`
	Target *struct {
		Entity struct {
			Domain *string `json:"domain"`
		} `json:"entity"`
	} `json:"target"`
}

func (c *WebsocketClient) GetServices(ctx context.Context) (ServiceList, error) {
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{Type: "get_services"})
	if err != nil {
		return nil, err
	}

	raw, err := wsGetResult(resp)
	if err != nil {
		return nil, err
	}

	var ret ServiceList
	err = json.Unmarshal(raw, &ret)

	return ret, nil
}

func (c *WebsocketClient) CallService(ctx context.Context,
	domain, service string, serviceData map[string]any, targetEntityId string) error {
	req := wsMsgCommonReq{
		Type:        "call_service",
		Domain:      domain,
		Service:     service,
		ServiceData: serviceData,
	}

	if targetEntityId != "" {
		req.Target = &wsMsgCommonReqTarget{
			EntityId: targetEntityId,
		}
	}

	resp, err := c.wsCommonCall(ctx, req)
	if err != nil {
		return err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return err
	}

	return nil
}

type ChangedState struct {
	EntityId  string    `json:"entity_id"`
	NewState  Entity    `json:"new_state"`
	OldState  Entity    `json:"old_state"`
	EventType string    `json:"event_type"`
	TimeFired time.Time `json:"time_fired"`
	Origin    string    `json:"origin"`
	Context   any       `json:"context"`
}

func (c *WebsocketClient) SubscribeEventsStateChanged(ctx context.Context) (<-chan ChangedState, error) {
	c.subscriptionMutex.Lock()
	defer c.subscriptionMutex.Unlock()

	if c.subscriptionEventCh != nil {
		return nil, errors.New("you have already subscribed to events")
	}

	ch := make(chan ChangedState)
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{
		Type:      "subscribe_events",
		EventType: "state_changed",
	})
	if err != nil {
		return nil, err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return nil, err
	}

	c.subscriptionEventId = resp.ID
	c.subscriptionEventCh = ch

	return ch, nil
}

func (c *WebsocketClient) UnsubscribeEventsStateChanged(ctx context.Context) error {
	c.subscriptionMutex.Lock()
	defer c.subscriptionMutex.Unlock()

	if c.subscriptionEventCh == nil {
		return errors.New("you haven't subscribed to events")
	}

	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{
		Type:         "unsubscribe_events",
		Subscription: c.subscriptionEventId,
	})
	if err != nil {
		return err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return err
	}

	close(c.subscriptionEventCh)
	c.subscriptionEventCh = nil

	return nil
}
