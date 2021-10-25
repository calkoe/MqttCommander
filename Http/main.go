package Http

import (
	"MqttCommander/Config"
	"bytes"
	"html/template"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func Deploy() {

	log.Info("[HTTP] Initialize Constraints and Actions")

	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Setup Actions
		for Actions_k := range Automation.Actions {
			Action := &Automation.Actions[Actions_k]

			if Action.Http != "" && !Action.Initialized {
				Action.Initialized = true

				// Setup Trigger Handler
				Action.Trigger = func() {
					Automation_c := Automation
					Action_c := Action
					tmpl, _ := template.New("value").Parse(Action_c.Http)
					var buf bytes.Buffer
					tmpl.Execute(&buf, Automation_c)
					http.Get(buf.String())
				}

			}

		}

	}

	log.Info("[HTTP] Initializiation completed!")

}
