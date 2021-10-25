package Cron

import (
	"MqttCommander/Config"
	"regexp"
	"time"

	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
)

var Constraint_regxp = regexp.MustCompile(`\s*(?P<Cron>[a-zA-Z0-9@*\s]*)(?:\s*-Reset\s+(?P<Reset>[[:digit:]nsusmssmh]+))?`)

func Deploy() {

	log.Info("[CRON] Initialize Constraints and Actions")

	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Setup Constraints
		for Constraint_k := range Automation.Constraints {
			Constraint := &Automation.Constraints[Constraint_k]

			if Constraint.Cron != "" && !Constraint.Initialized {
				Constraint.Initialized = true

				match := Constraint_regxp.FindStringSubmatch(Constraint.Cron)

				if len(match) == 3 {
					var err error
					Constraint.Cron_Parsed.Expression, err = cronexpr.Parse(match[1])
					Constraint.Cron_Parsed.Reset, _ = time.ParseDuration(match[2])
					if err != nil {
						return
					}
				}

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
							setTriggered(Constraint_c, false)
							Config.CheckTriggered(Automation_c)
						}
					}()
				}

				// Add Cron Trigger
				Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now())
				Constraint.Cron_Parsed.Cron_Timer = time.NewTimer(time.Until(Constraint.Cron_Parsed.NextTime))
				go func() {
					Automation_c := Automation
					Constraint_c := Constraint
					for {
						<-Constraint.Cron_Parsed.Cron_Timer.C
						if !Constraint_c.Initialized {
							Constraint_c.Cron_Parsed.Cron_Timer.Stop()
							return
						}
						setTriggered(Constraint_c, true)
						Config.CheckTriggered(Automation_c)
						Constraint.Cron_Parsed.NextTime = Constraint.Cron_Parsed.Expression.Next(time.Now())
						Constraint.Cron_Parsed.Cron_Timer.Reset(time.Until(Constraint.Cron_Parsed.NextTime))
					}
				}()

			}

		}

	}

	log.Info("[CRON] Initializiation completed!")

}

func setTriggered(Constraint *Config.Constraint_t, triggered bool) {

	if triggered {

		// Set Last Triggered
		Constraint.Triggered_Time = time.Now()

		// Reset Reset Timer
		if Constraint.Cron_Parsed.Reset > 0 {
			Constraint.Cron_Parsed.Reset_Timer.Reset(Constraint.Cron_Parsed.Reset)
		}

	}

	Constraint.Triggered = triggered

}
