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

	// Get Rule
	rule, ok := Rule.Get(RuleId)
	if !ok {
		return
	}
	module := rule.Module.(Mqtt_Parsed_t)

	// Get Automation
	automation, ok := Automation.Get(rule.AutomationId)
	if !ok {
		return
	}

	// Run Action
	if rule.Triggered != module.Reverse {

		// Run Template if exists
		var value interface{}
		if module.Template != nil {
			var buf bytes.Buffer
			module.Template.Execute(&buf, automation)
			value = Config.ParseType(buf.String())
		} else {
			value = module.Value
		}

		// Format Value
		var payload string
		if module.Object != "" {
			payload, _ = sjson.Set("{}", module.Object, value)
		} else {
			payload = fmt.Sprintf("%v", value)
		}

		// Publish payload
		if !Config.Get().Muted {
			Client.Publish(module.Topic, 2, module.Retained, payload)
		}
	}

}
