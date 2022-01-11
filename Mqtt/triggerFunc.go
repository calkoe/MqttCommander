package Mqtt

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"bytes"
	"fmt"

	"github.com/tidwall/sjson"
)

func TriggerFunc(RuleId uint) {

	// Get Action
	rule, ok := Rule.Get(RuleId)
	if !ok {
		return
	}
	module := rule.Module.(Mqtt_Parsed_t)

	// Get Parent Automation
	automation, ok := Automation.Get(rule.AutomationId)
	if !ok {
		return
	}

	// Run Action
	if rule.Triggered != module.Reverse {

		// Run Template if exists
		var value string
		if module.Template != nil {
			var buf bytes.Buffer
			module.Template.Execute(&buf, automation)
			value = buf.String()
		} else {
			value = fmt.Sprintf("%v", module.Value)
		}

		// Embedd into Object
		var payload string
		if module.Object != "" {
			payload, _ = sjson.Set("{}", module.Object, value)
		} else {
			payload = value
		}

		// Publish payload
		if !Config.Get().Muted {
			Client.Publish(module.Topic, 2, module.Retained, payload)
		}
	}

	// Stop RTT Measurement
	Automation.RTTstop(automation.Id)

}
