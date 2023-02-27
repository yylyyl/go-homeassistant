package homeassistant

type TriggerConfig interface {
	GetTriggerConfig() any
}

type StateTriggerConfig map[string]any

func NewStateTriggerConfig() StateTriggerConfig {
	return map[string]any{
		"platform": "state",
	}
}

func (c StateTriggerConfig) GetTriggerConfig() any {
	return c
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

type MultipleTriggerConfig struct {
	triggers []any
}

func NewMultipleTriggerConfig() *MultipleTriggerConfig {
	return &MultipleTriggerConfig{}
}

func (c *MultipleTriggerConfig) AddTrigger(t TriggerConfig) {
	c.triggers = append(c.triggers, t.GetTriggerConfig())
}

func (c *MultipleTriggerConfig) GetTriggerConfig() any {
	return c.triggers
}
