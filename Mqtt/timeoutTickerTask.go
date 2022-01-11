package Mqtt

import "MqttCommander/Rule"

func TimeoutTickerTask(RuleId uint) {
	rule, ok := Rule.Get(RuleId)
	for ok {
		<-rule.Module.(Mqtt_Parsed_t).Timeout_Ticker.C
		_, ok := Rule.Get(RuleId)
		if ok {
			SetTrigger(rule.Id, true)
		} else {
			break
		}
	}
}
