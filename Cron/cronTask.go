package Cron

import (
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"time"
)

func CronTask(RuleId uint) {
	config := Config.Get()
	rule, ok := Rule.Get(RuleId)
	for ok {
		<-rule.Module.(Cron_Parsed_t).Cron_Timer.C
		_, ok := Rule.Get(RuleId)
		if ok {
			SetTrigger(RuleId, true)
			module := rule.Module.(Cron_Parsed_t)
			module.NextTime = module.Expression.Next(time.Now().In(config.Timezone_parsed))
			module.Cron_Timer.Reset(module.NextTime.Sub(time.Now().In(config.Timezone_parsed)))
			Rule.SetModule(RuleId, module)
			// Reset immediately if no Reset timer defined
			if rule.Module.(Cron_Parsed_t).Reset == 0 {
				SetTrigger(rule.Id, false)
			}
		} else {
			break
		}
	}
}
