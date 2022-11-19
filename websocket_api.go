package homeassistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

func (c *WebsocketClient) Ping(ctx context.Context) error {
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{Type: "ping"})
	if err != nil {
		return err
	}

	if resp.Type != "pong" {
		return fmt.Errorf("unexpected response of ping: %s", resp.Type)
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
	EntityId string `json:"entity_id"`
	NewState Entity `json:"new_state"`
	OldState Entity `json:"old_state"`
}

func (c *WebsocketClient) SubscribeToChangedStates(ctx context.Context) (<-chan ChangedState, uint, error) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	ch := make(chan ChangedState)
	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{
		Type:      "subscribe_events",
		EventType: "state_changed",
	})
	if err != nil {
		return nil, 0, err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return nil, 0, err
	}

	c.eventMap[resp.ID] = &eventReceiver{
		eventType: eventTypeState,
		stateCh:   ch,
	}

	return ch, resp.ID, nil
}

// SubscribeToTrigger subscribes to triggers, check documents
// here https://www.home-assistant.io/docs/automation/trigger/
func (c *WebsocketClient) SubscribeToTrigger(ctx context.Context, trigger TriggerConfig) (<-chan []byte, uint, error) {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	ch := make(chan []byte)

	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{
		Type:    "subscribe_trigger",
		Trigger: trigger,
	})
	if err != nil {
		return nil, 0, err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return nil, 0, err
	}

	c.eventMap[resp.ID] = &eventReceiver{
		eventType: eventTypeTrigger,
		triggerCh: ch,
	}

	return ch, resp.ID, nil
}

// UnsubscribeToEvents can be used to unsubscribe to states or triggers
func (c *WebsocketClient) UnsubscribeToEvents(ctx context.Context, id uint) error {
	c.eventMutex.Lock()
	defer c.eventMutex.Unlock()

	receiver := c.eventMap[id]
	if receiver == nil {
		return errors.New("invalid subscription id")
	}

	resp, err := c.wsCommonCall(ctx, wsMsgCommonReq{
		Type:         "unsubscribe_events",
		Subscription: id,
	})
	if err != nil {
		return err
	}

	_, err = wsGetResult(resp)
	if err != nil {
		return err
	}

	switch receiver.eventType {
	case eventTypeState:
		close(receiver.stateCh)
	case eventTypeTrigger:
		close(receiver.triggerCh)
	default:
		log.Printf("WARN unimplemented event: %d", receiver.eventType)
	}
	delete(c.eventMap, id)

	return nil
}
