package Http

import (
	"MqttCommander/Config"
	"bytes"
	"html/template"
	"net/http"
)

func Deploy() {

	for _, Automation := range Config.GetAutomations() {

		// Setup Actions
		for Action_k := range Automation.Actions {
			Automation.Actions[Action_k].Mutex.Lock()
			defer Automation.Actions[Action_k].Mutex.Unlock()

			Action := &Automation.Actions[Action_k]
			if Action.Http != "" && !Action.Initialized {
				Action.Initialized = true
				// Setup Trigger Handler
				Action.Trigger = func(Automation Config.Automation_t, Action *Config.Action_t) {
					tmpl, _ := template.New("value").Parse(Action.Http)
					var buf bytes.Buffer
					tmpl.Execute(&buf, Automation)
					http.Get(buf.String())
				}

			}

		}

	}

}
