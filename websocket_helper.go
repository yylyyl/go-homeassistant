package homeassistant

import (
	"encoding/json"
)

type TriggerConfig map[string]any
type StateTriggerConfig TriggerConfig

func NewStateTriggerConfig() StateTriggerConfig {
	return map[string]any{
		"platform": "state",
	}
}

func (c StateTriggerConfig) EntityId(v any) {
	c["entity_id"] = v
}

func (c StateTriggerConfig) To(v any) {
	c["to"] = v
}

func (c StateTriggerConfig) From(v any) {
	c["from"] = v
}

func (c StateTriggerConfig) NotFrom(v any) {
	c["not_from"] = v
}

func (c StateTriggerConfig) NotTo(v any) {
	c["not_to"] = v
}

func (c StateTriggerConfig) Attribute(v any) {
	c["attribute"] = v
}

func (c StateTriggerConfig) For(v any) {
	c["for"] = v
}

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
