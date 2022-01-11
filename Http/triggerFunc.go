package Http

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"bytes"
	"net/http"
)

func TriggerFunc(RuleId uint, context interface{}) {

	// Get Rule
	rule, ok := Rule.Get(RuleId)
	if !ok {
		return
	}
	module := rule.Module.(Http_Parsed_t)

	// Run Action
	if rule.Triggered != module.Reverse && module.Template != nil {

		var buf bytes.Buffer
		rule.Module.(Http_Parsed_t).Template.Execute(&buf, context.(Automation.Automation_t))
		if !Config.Get().Muted {
			http.Get(buf.String())
		}

	}

}
