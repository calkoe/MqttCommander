package Cron

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"strconv"
	"time"

	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
)

type Cron_Parsed_t struct {
	Expression  *cronexpr.Expression
	NextTime    time.Time
	Cron_Timer  *time.Timer
	Reset       time.Duration
	Reset_Timer *time.Timer
	NoTrigger   bool
}

func Begin() {

}

func Deploy() {

	config := Config.Get()

	// Setup Constraints
	for _, rule := range Rule.GetAllByTag("constraint/cron") {

		if !rule.Initialized {

			module := Cron_Parsed_t{}

			// Parse Arguments
			var err error
			module.Expression, err = cronexpr.Parse(Config.FindParm(`^\s*(?P<Value>[^\n\(]*)`, rule.Text))
			if err != nil {
				log.Errorf("[CRON] Error while parsing expression %s ", err)
				return
			}
			module.Reset, _ = time.ParseDuration(Config.FindParm(`\(Reset\s+(\S+)\)`, rule.Text))
			module.NoTrigger, _ = strconv.ParseBool(Config.FindParm(`\(NoTrigger\s+(\S+)\)`, rule.Text))

			// Add Reset Timer
			if module.Reset > 0 {
				module.Reset_Timer = time.NewTimer(module.Reset)
				module.Reset_Timer.Stop()
			}

			// Add Cron SetTrigger
			module.NextTime = module.Expression.Next(time.Now().In(config.Timezone_parsed))
			module.Cron_Timer = time.NewTimer(module.NextTime.Sub(time.Now().In(config.Timezone_parsed)))

			// Save Changes
			Rule.SetModule(rule.Id, module)

			// Add Reset Timer Task
			if module.Reset_Timer != nil {
				go ResetTimerTask(rule.Id)
			}

			// Add Cron Task
			go CronTask(rule.Id)

		}

	}

}

func SetTrigger(RuleId uint, trigger bool) {

	rule, ok := Rule.Get(RuleId)
	if ok {

		module := rule.Module.(Cron_Parsed_t)

		// Reset Reset Timer
		if trigger && module.Reset > 0 {
			module.Reset_Timer.Reset(module.Reset)
		}

		Rule.SetTrigger(RuleId, trigger)
		Automation.CheckTriggered(rule.AutomationId, rule.Module.(Cron_Parsed_t).NoTrigger)

	}

}
