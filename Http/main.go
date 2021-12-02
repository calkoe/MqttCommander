package Http

import (
	"MqttCommander/Config"
	"bytes"
	"html/template"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func Deploy() {

	for _, AutomationCopy := range Config.CopyAutomations() {

		// Setup Actions
		for Action_k := range AutomationCopy.Actions {

			ActionCopy := Config.CopyAction(&AutomationCopy.Actions[Action_k])
			if ActionCopy.Http != "" && !ActionCopy.Initialized {

				AutomationCopy.Actions[Action_k].Mutex.Lock()
				Action := &AutomationCopy.Actions[Action_k]

				Action.Initialized = true

				// Parse Template
				var err error
				Action.Http_Parsed.Template, err = template.New("value").Parse(ActionCopy.Http)
				if err != nil {
					log.Errorf("[HTTP] error while parsing Template: %s", err)
					return
				}

				// Setup Trigger Handler
				Action.Trigger = func(AutomationCopy Config.Automation_t, ActionCopy Config.Action_t) {

					// Run Action
					if ActionCopy.Triggered && Action.Http_Parsed.Template != nil {
						var buf bytes.Buffer
						Action.Http_Parsed.Template.Execute(&buf, AutomationCopy)
						http.Get(buf.String())
					}

					// Stop RTT Measurement
					Config.RTTstop(AutomationCopy.Id)

				}

				AutomationCopy.Actions[Action_k].Mutex.Unlock()

			}

		}

	}

}
