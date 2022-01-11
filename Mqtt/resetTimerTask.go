package Mqtt

import "MqttCommander/Rule"

func ResetTimerTask(RuleId uint) {
	rule, ok := Rule.Get(RuleId)
	for ok {
		<-rule.Module.(Mqtt_Parsed_t).Reset_Timer.C
		_, ok := Rule.Get(RuleId)
		if ok {
			SetTrigger(rule.Id, false)
		} else {
			break
		}
	}
}
