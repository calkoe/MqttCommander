package Http

import (
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"html/template"
	"regexp"
	"strconv"

	log "github.com/sirupsen/logrus"
)

type Http_Parsed_t struct {
	Url      string
	Reverse  bool
	Template *template.Template
}

var Expression *regexp.Regexp

func Begin() {

	Expression = regexp.MustCompile(`^(?P<Url>[^\n\(]*)`)

}

func Deploy() {

	// Setup Actions
	for _, rule := range Rule.GetAllByTag("action/mqtt") {

		if !rule.Initialized {

			module := Http_Parsed_t{}

			// Parse Arguments
			match := Expression.FindStringSubmatch(rule.Text)
			if len(match) == 2 {

				module.Url = match[1]
				module.Reverse, _ = strconv.ParseBool(Config.FindParm(`\(Reverse\s+(\S+)\)`, rule.Text))

				// Prepare Template
				var err error
				module.Template, err = template.New("value").Parse(rule.Text)
				if err != nil {
					log.Errorf("[HTTP] error while parsing Template: %s", err)
					return
				}

				// Save Changes
				Rule.SetModule(rule.Id, module)

				// Setup SetTrigger Handler
				Rule.SetTriggerFunc(rule.Id, TriggerFunc)
			}

		}

	}

}
