package Mqtt

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"html/template"
	"regexp"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

type Mqtt_Parsed_t struct {
	Topic          string
	Object         string
	Comparator     string
	Value          interface{}
	Retained       bool
	Reverse        bool
	Reset          time.Duration
	Reset_Timer    *time.Timer
	Timeout        time.Duration
	Timeout_Ticker *time.Ticker
	NoTrigger      bool
	NoValue        bool
	Template       *template.Template
}

var Client MQTT.Client
var Expression *regexp.Regexp

//define a function for the default message handler
var DefaultPublishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	//fmt.Printf("TOPIC: %s\n", msg.Topic())
	//fmt.Printf("MSG: %s\n", msg.Payload())

	// Start RTT Measurement
	go MessageHandlerTask(time.Now(), client, msg)

}

func Begin() {

	ConfigCopy := Config.Get()

	Expression = regexp.MustCompile(`^\s*(?P<Topic>[^\.\s]*)(?:\.(?P<Object>\S*))?(?:\s+(?P<Comparator>[^\n\s\(]*))?(?:\s+(?P<Value>[^\n\(]*))?`)

	log.Infof("[MQTT] Connecting to MQTT Server: %s", ConfigCopy.MqttUri)

	// Create Client
	opts := MQTT.NewClientOptions()
	opts.AddBroker(ConfigCopy.MqttUri)
	opts.SetClientID(ConfigCopy.Name)

	// Set Reconnect
	opts.SetKeepAlive(5 * time.Second)
	opts.SetPingTimeout(5 * time.Second)
	opts.SetResumeSubs(true)
	opts.SetCleanSession(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Second)

	// Set Handlers
	opts.SetDefaultPublishHandler(DefaultPublishHandler)
	opts.SetOnConnectHandler(func(c MQTT.Client) {
		log.Info("[MQTT] Successfully connected to MQTT server! send subscriptions again")
		// Resubscripe
		for _, rule := range Rule.GetAllByTag("constraint/mqtt") {
			if rule.Initialized {
				go func(rule Rule.Rule_t) {
					if token := Client.Subscribe(rule.Module.(Mqtt_Parsed_t).Topic, 2, nil); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
						Rule.SetError(rule.Id, "[MQTT] Error while subscribing to topic %v ", token.Error())
					} else {
						Rule.SetError(rule.Id, "")
					}
				}(rule)
			}
		}
	})
	opts.SetConnectionLostHandler(func(c MQTT.Client, err error) {
		log.Errorf("[MQTT] Connection to server lost: %s", err)
	})
	opts.SetReconnectingHandler(func(c MQTT.Client, o *MQTT.ClientOptions) {
		log.Warn("[MQTT] Trying to reconnect to Server")
	})

	//create and start a client using the above ClientOptions
	Client = MQTT.NewClient(opts)
	if token := Client.Connect(); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		log.Errorf("[MQTT] Connection to MQTT server failed, error: %v", token.Error())
	}

}

// Init Constraints and Actions
func Deploy() {

	// Setup Constraints
	for _, rule := range Rule.GetAllByTag("constraint/mqtt") {

		if !rule.Initialized {

			module := Mqtt_Parsed_t{}

			// Debug
			/*
				fmt.Printf("Text: %s\n", Constraint_v.Mqtt)
				fmt.Printf("Found %d Matches!\n", len(match))
				for i, name := range myExp.SubexpNames() {
					if i != 0 && name != "" && i < len(match) {
						fmt.Printf("%s: %s\n", name, match[i])
					}
				}
			*/

			// Parse Arguments
			match := Expression.FindStringSubmatch(rule.Text)
			if len(match) == 5 {

				module.Topic = match[1]
				module.Object = match[2]
				module.Comparator = match[3]
				module.Value = Config.ParseType(match[4])

				module.Reset, _ = time.ParseDuration(Config.FindParm(`\(Reset\s+(\S+)\)`, rule.Text))
				module.Timeout, _ = time.ParseDuration(Config.FindParm(`\(Timeout\s+(\S+)\)`, rule.Text))
				module.Retained, _ = strconv.ParseBool(Config.FindParm(`\(BlockRetained\s+(\S+)\)`, rule.Text))
				module.NoTrigger, _ = strconv.ParseBool(Config.FindParm(`\(NoTrigger\s+(\S+)\)`, rule.Text))
				module.NoValue, _ = strconv.ParseBool(Config.FindParm(`\(NoValue\s+(\S+)\)`, rule.Text))

				// Add Reset Timer
				if module.Reset > 0 {
					module.Reset_Timer = time.NewTimer(module.Reset)
					module.Reset_Timer.Stop()
				}

				// Add Timeout Ticker
				if module.Timeout > 0 {
					module.Timeout_Ticker = time.NewTicker(module.Timeout)
				}

				// Save Changes
				Rule.SetModule(rule.Id, module)

				// Add Subscription
				go func(rule Rule.Rule_t) {
					if token := Client.Subscribe(module.Topic, 2, nil); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
						Rule.SetError(rule.Id, "[MQTT] Error while subscribing to topic %v ", token.Error())
					} else {
						Rule.SetError(rule.Id, "")
					}
				}(rule)

				// Add Reset Timer Task
				if module.Reset_Timer != nil {
					go ResetTimerTask(rule.Id)
				}

				// Add Timeout Ticker Task
				if module.Timeout_Ticker != nil {
					go TimeoutTickerTask(rule.Id)
				}

			}

		}

	}

	// Setup Actions
	for _, rule := range Rule.GetAllByTag("action/mqtt") {

		if !rule.Initialized {

			module := Mqtt_Parsed_t{}

			// Debug
			/*
				fmt.Printf("Text: %s\n", Action.Mqtt)
				fmt.Printf("Found %d Matches!\n", len(match))
				for i, name := range myExp.SubexpNames() {
					if i != 0 && name != "" && i < len(match) {
						fmt.Printf("%s: %s\n", name, match[i])
					}
				}
			*/

			match := Expression.FindStringSubmatch(rule.Text)
			if len(match) == 5 {

				module.Topic = match[1]
				module.Object = match[2]
				module.Value = Config.ParseType(match[4])

				module.Retained, _ = strconv.ParseBool(Config.FindParm(`\(Retained\s+(\S+)\)`, rule.Text))
				module.Reverse, _ = strconv.ParseBool(Config.FindParm(`\(Reverse\s+(\S+)\)`, rule.Text))

				// Parse Template
				switch module.Value.(type) {
				case string:
					var err error
					module.Template, err = template.New("value").Parse(module.Value.(string))
					if err != nil {
						Rule.SetError(rule.Id, "[MQTT] error while parsing Template: %v", err)
						return
					} else {
						Rule.SetError(rule.Id, "")
					}
				}

				// Save Changes
				Rule.SetModule(rule.Id, module)

				// Setup SetTrigger Handler
				Rule.SetTriggerFunc(rule.Id, TriggerFunc)
			}

		}

	}

}

func SetTrigger(RuleId uint, trigger bool) {

	rule, ok := Rule.Get(RuleId)
	if ok {

		module := rule.Module.(Mqtt_Parsed_t)

		// Reset Reset Timer
		if trigger && module.Reset > 0 {
			module.Reset_Timer.Reset(module.Reset)
		}

		Rule.SetTrigger(RuleId, trigger)
		Automation.CheckTriggered(rule.AutomationId, module.NoTrigger)

	}

}
