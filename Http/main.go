package Http

import (
	"MqttCommander/Config"
	"bytes"
	"html/template"
	"net/http"
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

				// Setup Trigger Handler
				Action.Trigger = func(AutomationCopy Config.Automation_t, ActionCopy Config.Action_t) {
					tmpl, _ := template.New("value").Parse(ActionCopy.Http)
					var buf bytes.Buffer
					tmpl.Execute(&buf, AutomationCopy)
					http.Get(buf.String())
				}

				AutomationCopy.Actions[Action_k].Mutex.Unlock()

			}

		}

	}

}
