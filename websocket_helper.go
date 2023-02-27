package homeassistant

import (
	"context"
	"encoding/json"
	"log"
)

type TriggeredState struct {
	Platform    string `json:"platform"`
	EntityId    string `json:"entity_id"`
	FromState   Entity `json:"from_state"`
	ToState     Entity `json:"to_state"`
	For         any    `json:"for"`
	Attribute   any    `json:"attribute"`
	Description string `json:"description"`
}

type triggeredState struct {
	Trigger *TriggeredState `json:"trigger"`
}

func ParseTriggeredState(input []byte) (*TriggeredState, error) {
	var ret triggeredState
	err := json.Unmarshal(input, &ret)
	if err != nil {
		return nil, err
	}
	return ret.Trigger, nil
}

func (c *WebsocketClient) SubscribeToStateTrigger(ctx context.Context, trigger StateTriggerConfig) (<-chan *TriggeredState, uint, error) {
	ch, id, err := c.SubscribeToTrigger(ctx, trigger)
	if err != nil {
		return nil, 0, err
	}

	newCh := make(chan *TriggeredState)
	go func() {
		defer close(newCh)
		for raw := range ch {
			st, err := ParseTriggeredState(raw)
			if err != nil {
				log.Printf("WARN failed to parse triggered state: %v %s", err, raw)
				continue
			}
			newCh <- st
		}
	}()

	return newCh, id, nil
}

func (c *WebsocketClient) GetStatesMap(ctx context.Context) (map[string]*Entity, error) {
	list, err := c.GetStates(ctx)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]*Entity)
	for _, e := range list {
		ret[e.EntityId] = e
	}

	return ret, nil
}
