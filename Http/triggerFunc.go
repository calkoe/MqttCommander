package Http

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"bytes"
	"net/http"
)

func TriggerFunc(RuleId uint) {

	// Get Action
	rule, ok := Rule.Get(RuleId)
	if !ok {
		return
	}

	// Get Parent Automation
	automation, ok := Automation.Get(rule.AutomationId)
	if !ok {
		return
	}

	// Run Action
	if rule.Triggered != rule.Module.(Http_Parsed_t).Reverse && rule.Module.(Http_Parsed_t).Template != nil {
		var buf bytes.Buffer
		rule.Module.(Http_Parsed_t).Template.Execute(&buf, automation)
		if !Config.Get().Muted {
			http.Get(buf.String())
		}
	}

	// Stop RTT Measurement
	Automation.RTTstop(rule.AutomationId)

}
