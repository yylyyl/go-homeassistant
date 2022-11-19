package homeassistant

import "testing"

func TestParseTriggeredState(t *testing.T) {
	input := []byte(`{"trigger":{"id":"0","idx":"0","alias":null,"platform":"state","entity_id":"light.lightbedroom","from_state":{"entity_id":"light.lightbedroom","state":"off","attributes":{"friendly_name":"LightBedroom","supported_features":44},"last_changed":"2022-11-19T01:46:15.996018+00:00","last_updated":"2022-11-19T01:46:16.015168+00:00","context":{"id":"12345","parent_id":null,"user_id":null}},"to_state":{"entity_id":"light.lightbedroom","state":"on","attributes":{"friendly_name":"LightBedroom","supported_features":44},"last_changed":"2022-11-19T01:49:54.404769+00:00","last_updated":"2022-11-19T01:49:54.404769+00:00","context":{"id":"67890","parent_id":null,"user_id":null}},"for":null,"attribute":null,"description":"state of light.lightbedroom"}}`)

	ts, err := ParseTriggeredState(input)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if ts.EntityId != "light.lightbedroom" || ts.ToState.State != "on" {
		t.Fatalf("unexpected result: %v", ts)
	}
}
