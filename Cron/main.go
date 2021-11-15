package Cron

import (
	"MqttCommander/Config"
	"strconv"
	"time"

	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
)

func Deploy() {

	log.Info("[CRON] Initialize Constraints and Actions")

	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Setup Constraints
		for Constraint_k := range Automation.Constraints {
			Constraint := &Automation.Constraints[Constraint_k]

			if Constraint.Cron != "" && !Constraint.Initialized {
				Constraint.Initialized = true

				Constraint.Cron_Parsed.Expression, _ = cronexpr.Parse(Config.Find(`^\s*((?:[^-][^\s]*\s?)+|@[a-z]+)`, Constraint.Cron))
				Constraint.Cron_Parsed.Reset, _ = time.ParseDuration(Config.Find(`-Reset\s+(\S+)`, Constraint.Cron))
				Constraint.Cron_Parsed.NoTrigger, _ = strconv.ParseBool(Config.Find(`-NoTrigger\s+(\S+)`, Constraint.Cron))

				// Add Reset Timer
				if Constraint.Cron_Parsed.Reset > 0 {
					Constraint.Cron_Parsed.Reset_Timer = time.NewTimer(Constraint.Cron_Parsed.Reset)
					Constraint.Cron_Parsed.Reset_Timer.Stop()
					go func() {
						Automation_c := Automation
						Constraint_c := Constraint
						for {
							<-Constraint_c.Cron_Parsed.Reset_Timer.C
							if !Constraint_c.Initialized {
								Constraint_c.Cron_Parsed.Reset_Timer.Stop()
								return
							}
							setTriggered(Automation_c, Constraint_c, false)
						}
					}()
				}

				// Add Cron Trigger
				Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now().In(Config.Config.Timezone_parsed))
				Constraint.Cron_Parsed.Cron_Timer = time.NewTimer(Constraint.Cron_Parsed.NextTime.Sub(time.Now().In(Config.Config.Timezone_parsed)))
				go func() {
					Automation_c := Automation
					Constraint_c := Constraint
					for {
						<-Constraint.Cron_Parsed.Cron_Timer.C
						if !Constraint_c.Initialized {
							Constraint_c.Cron_Parsed.Cron_Timer.Stop()
							return
						}
						setTriggered(Automation_c, Constraint_c, true)
						Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now().In(Config.Config.Timezone_parsed))
						Constraint.Cron_Parsed.Cron_Timer.Reset(Constraint.Cron_Parsed.NextTime.Sub(time.Now().In(Config.Config.Timezone_parsed)))
						// Reset immediately if no Reset timer defined
						if Constraint.Cron_Parsed.Reset == 0 {
							setTriggered(Automation_c, Constraint_c, false)
						}
					}
				}()

			}

		}

	}

	log.Info("[CRON] Initializiation completed!")

}

func setTriggered(Automation *Config.Automation_t, Constraint *Config.Constraint_t, triggered bool) {

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
	Config.CheckTriggered(Automation, Constraint.Mqtt_Parsed.NoTrigger)

}
