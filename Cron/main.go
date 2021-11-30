package Cron

import (
	"MqttCommander/Config"
	"strconv"
	"time"

	"github.com/gorhill/cronexpr"
)

func Deploy() {

	ConfigFileCopy := Config.CopyConfigFile()

	for id, AutomationCopy := range Config.CopyAutomations() {

		// Setup Constraints
		for Constraint_k := range AutomationCopy.Constraints {

			ConstraintCopy := Config.CopyConstraint(&AutomationCopy.Constraints[Constraint_k])
			if ConstraintCopy.Cron != "" && !ConstraintCopy.Initialized {

				AutomationCopy.Constraints[Constraint_k].Mutex.Lock()
				Constraint := &AutomationCopy.Constraints[Constraint_k]

				Constraint.Initialized = true

				Constraint.Cron_Parsed.Expression, _ = cronexpr.Parse(Config.Find(`^\s*((?:[^-][^\s]*\s?)+|@[a-z]+)`, Constraint.Cron))
				Constraint.Cron_Parsed.Reset, _ = time.ParseDuration(Config.Find(`-Reset\s+(\S+)`, Constraint.Cron))
				Constraint.Cron_Parsed.NoTrigger, _ = strconv.ParseBool(Config.Find(`-NoTrigger\s+(\S+)`, Constraint.Cron))

				// Add Reset Timer
				if Constraint.Cron_Parsed.Reset > 0 {
					Constraint.Cron_Parsed.Reset_Timer = time.NewTimer(Constraint.Cron_Parsed.Reset)
					Constraint.Cron_Parsed.Reset_Timer.Stop()
					go func(id int, Constraint *Config.Constraint_t) {
						_, ok := Config.CopyAutomation(id)
						for ok {
							<-Constraint.Cron_Parsed.Reset_Timer.C
							_, ok := Config.CopyAutomation(id)
							if ok {
								Constraint.Mutex.Lock()
								setTriggered(id, Constraint, false)
								Constraint.Mutex.Unlock()
							} else {
								break
							}
						}
					}(id, Constraint)
				}

				// Add Cron Trigger
				Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now().In(ConfigFileCopy.Timezone_parsed))
				Constraint.Cron_Parsed.Cron_Timer = time.NewTimer(Constraint.Cron_Parsed.NextTime.Sub(time.Now().In(ConfigFileCopy.Timezone_parsed)))
				go func(id int, Constraint *Config.Constraint_t) {
					_, ok := Config.CopyAutomation(id)
					for ok {
						<-Constraint.Cron_Parsed.Cron_Timer.C
						_, ok := Config.CopyAutomation(id)
						if ok {
							Constraint.Mutex.Lock()
							setTriggered(id, Constraint, true)
							Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now().In(ConfigFileCopy.Timezone_parsed))
							Constraint.Cron_Parsed.Cron_Timer.Reset(Constraint.Cron_Parsed.NextTime.Sub(time.Now().In(ConfigFileCopy.Timezone_parsed)))
							// Reset immediately if no Reset timer defined
							if Constraint.Cron_Parsed.Reset == 0 {
								setTriggered(id, Constraint, false)
							}
							Constraint.Mutex.Unlock()
						} else {
							break
						}
					}
				}(id, Constraint)

				AutomationCopy.Constraints[Constraint_k].Mutex.Unlock()

			}

		}

	}

}

func setTriggered(id int, Constraint *Config.Constraint_t, triggered bool) {

	if triggered {

		// Set Last Triggered
		Constraint.Triggered_Time = time.Now()

		// Reset Reset Timer
		if Constraint.Cron_Parsed.Reset > 0 {
			Constraint.Cron_Parsed.Reset_Timer.Reset(Constraint.Cron_Parsed.Reset)
		}

	}

	Constraint.Triggered = triggered

	// CheckTriggered
	go Config.CheckTriggered(id, Constraint.Mqtt_Parsed.NoTrigger)

}
