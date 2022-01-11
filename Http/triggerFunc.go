package Http

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"bytes"
	"net/http"
)

func TriggerFunc(RuleId uint) {

	// Get Rule
	rule, ok := Rule.Get(RuleId)
	if !ok {
		return
	}
	module := rule.Module.(Http_Parsed_t)

	// Get Automation
	automation, ok := Automation.Get(rule.AutomationId)
	if !ok {
		return
	}

	// Run Action
	if rule.Triggered != module.Reverse && module.Template != nil {

		var buf bytes.Buffer
		rule.Module.(Http_Parsed_t).Template.Execute(&buf, automation)
		if !Config.Get().Muted {
			http.Get(buf.String())
		}

	}

}
