package Mqtt

import (
	"MqttCommander/Config"
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
)

/*
type subscription_t struct {
	Topic      string
	Comparator string
	Value      string
	Reset      uint
	Timeout    uint
	NoTrigger  bool
	Retained   bool
	Error      error
}
*/

var Client MQTT.Client

//define a function for the default message handler
var DefaultPublishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	//fmt.Printf("TOPIC: %s\n", msg.Topic())
	//fmt.Printf("MSG: %s\n", msg.Payload())

	// Find matching topics
	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Find matching topics
		for Constraint_k := range Automation.Constraints {
			Constraint := &Automation.Constraints[Constraint_k]
			if Constraint.Mqtt != "" && Constraint.Mqtt_Parsed.Topic == msg.Topic() {
				onMessage(Automation, Constraint, client, msg)
			}
		}

	}
}

func Connect() {

	log.Infof("[MQTT] Connecting to MQTT Server: %s", Config.Config.MqttUri)

	// Create Client
	opts := MQTT.NewClientOptions()
	opts.AddBroker(Config.Config.MqttUri)
	opts.SetClientID(Config.Config.Name)

	// Set Reconnect
	opts.SetKeepAlive(5 * time.Second)
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
		for _, Automation := range Config.Config.Automations {
			for _, Constraint := range Automation.Constraints {
				if Constraint.Mqtt != "" && Constraint.Initialized {
					Constraint.Mqtt_Parsed.Token = Client.Subscribe(Constraint.Mqtt_Parsed.Topic, 2, nil)
				}
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
	token := Client.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Errorf("[MQTT] Connection to MQTT server failed, error: %s", token.Error())
	}

}

// Init Constraints and Actions
func Deploy() {

	log.Info("[MQTT] Initialize Constraints and Actions")

	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Setup Constraints
		for Constraint_k := range Automation.Constraints {
			Constraint := &Automation.Constraints[Constraint_k]

			if Constraint.Mqtt != "" && !Constraint.Initialized {
				Constraint.Initialized = true

				match := regexp.MustCompile(`^\s*(?P<Topic>[^\.\s]*)(?:\.(?P<Object>\S*))?(?:\s+(?P<Comparator>[^-][\S]*))?(?:\s+"(?P<Value_String>.*)")?(?:\s+(?P<Value_Float>(?:-[0-9.]+)|(?:[^-][\S]*)))?.*$`).FindStringSubmatch(Constraint.Mqtt)

				// Debug
				/*
					fmt.Printf("Text: %s\n", Constraint.Mqtt)
					fmt.Printf("Found %d Matches!\n", len(match))
					for i, name := range myExp.SubexpNames() {
						if i != 0 && name != "" && i < len(match) {
							fmt.Printf("%s: %s\n", name, match[i])
						}
					}
				*/

				// Parse Arguments
				if len(match) == 6 {

					Constraint.Mqtt_Parsed.Topic = match[1]
					Constraint.Mqtt_Parsed.Object = match[2]
					Constraint.Mqtt_Parsed.Comparator = match[3]
					// Value_String
					if match[4] != "" {
						Constraint.Mqtt_Parsed.Value = match[4]
					}
					// Value_Float
					if match[5] != "" {
						Constraint.Mqtt_Parsed.Value, _ = strconv.ParseFloat(match[5], 64)
					}
					Constraint.Mqtt_Parsed.Reset, _ = time.ParseDuration(Config.Find(`-Reset\s+(\S+)`, Constraint.Mqtt))
					Constraint.Mqtt_Parsed.Timeout, _ = time.ParseDuration(Config.Find(`-Timeout\s+(\S+)`, Constraint.Mqtt))
					Constraint.Mqtt_Parsed.BlockRetained, _ = strconv.ParseBool(Config.Find(`-BlockRetained\s+(\S+)`, Constraint.Mqtt))
					Constraint.Mqtt_Parsed.NoTrigger, _ = strconv.ParseBool(Config.Find(`-NoTrigger\s+(\S+)`, Constraint.Mqtt))
					Constraint.Mqtt_Parsed.NoValue, _ = strconv.ParseBool(Config.Find(`-NoValue\s+(\S+)`, Constraint.Mqtt))

					// Add Subscription
					Constraint.Mqtt_Parsed.Token = Client.Subscribe(Constraint.Mqtt_Parsed.Topic, 2, nil)

					// Add Reset Timer
					if Constraint.Mqtt_Parsed.Reset > 0 {
						Constraint.Mqtt_Parsed.Reset_Timer = time.NewTimer(Constraint.Mqtt_Parsed.Reset)
						Constraint.Mqtt_Parsed.Reset_Timer.Stop()
						go func() {
							Constraint_c := Constraint
							Automation_c := Automation
							for {
								<-Constraint_c.Mqtt_Parsed.Reset_Timer.C
								if !Constraint_c.Initialized {
									Constraint_c.Mqtt_Parsed.Reset_Timer.Stop()
									return
								}
								setTriggered(Automation_c, Constraint_c, false)
							}
						}()
					}

					// Add Timeout Ticker
					if Constraint.Mqtt_Parsed.Timeout > 0 {
						Constraint.Mqtt_Parsed.Timeout_Ticker = time.NewTicker(Constraint.Mqtt_Parsed.Timeout)
						go func() {
							Constraint_c := Constraint
							Automation_c := Automation
							for {
								<-Constraint_c.Mqtt_Parsed.Timeout_Ticker.C
								if !Constraint_c.Initialized {
									Constraint_c.Mqtt_Parsed.Timeout_Ticker.Stop()
									return
								}
								setTriggered(Automation_c, Constraint_c, true)
							}
						}()
					}

				}

			}

		}

		// Setup Actions
		for Actions_k := range Automation.Actions {
			Action := &Automation.Actions[Actions_k]

			if Action.Mqtt != "" {

				match := regexp.MustCompile(`^\s*(?P<Topic>[^\.\s]*)(?:\.(?P<Object>\S*))?(?:\s+(?P<Comparator>[^-][\S]*))?(?:\s+"(?P<Value_String>.*)")?(?:\s+(?P<Value_Float>(?:-[0-9.]+)|(?:[^-][\S]*)))?.*$`).FindStringSubmatch(Action.Mqtt)

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

				if len(match) == 6 {

					Action.Mqtt_Parsed.Topic = match[1]
					Action.Mqtt_Parsed.Object = match[2]
					//Constraint.Mqtt_Parsed.Comparator = match[3]
					// Value_String
					if match[4] != "" {
						Action.Mqtt_Parsed.Value = match[4]
						Action.Mqtt_Parsed.IsString = true
					}
					// Value_Float
					if match[5] != "" {
						Action.Mqtt_Parsed.Value = match[5]
						Action.Mqtt_Parsed.IsString = false
					}
					Action.Mqtt_Parsed.Retained, _ = strconv.ParseBool(Config.Find(`-Retained\s+(\S+)`, Action.Mqtt))

					// Setup Trigger Handler
					Action.Trigger = func() {
						Automation_c := Automation
						Action_c := Action
						var payload string
						tmpl, err := template.New("value").Parse(Action_c.Mqtt_Parsed.Value)
						if err != nil {
							log.Errorf("[MQTT] error while parsing Template: %s", err)
						} else {
							var buf bytes.Buffer
							tmpl.Execute(&buf, Automation_c)
							if Action_c.Mqtt_Parsed.Object != "" {
								if Action.Mqtt_Parsed.IsString {
									payload = fmt.Sprintf("{\"%s\":\"%s\"}", Action.Mqtt_Parsed.Object, buf.String())
								} else {
									payload = fmt.Sprintf("{\"%s\":%s}", Action.Mqtt_Parsed.Object, buf.String())
								}
							} else {
								payload = buf.String()
							}
							// Publish
							Client.Publish(Action_c.Mqtt_Parsed.Topic, 2, Action_c.Mqtt_Parsed.Retained, payload)
						}

					}

				}

			}

		}

	}

	log.Info("[MQTT] Initializiation completed!")

}

func onMessage(Automation *Config.Automation_t, Constraint *Config.Constraint_t, client MQTT.Client, msg MQTT.Message) {

	// Debug
	// fmt.Printf("%s\n", msg.Payload())

	// Block Retained
	if Constraint.Mqtt_Parsed.BlockRetained && msg.Retained() {
		return
	}

	// Raw Value
	if Constraint.Mqtt_Parsed.Object == "" {

		// Set Constraint value
		parsed, err := strconv.ParseFloat(string(msg.Payload()), 64)
		if err == nil {
			Constraint.Value = parsed
		} else {
			Constraint.Value = string(msg.Payload())
		}
		Constraint.Value_Time = time.Now()

		// Don't set Automations value to Constraint value
		if !Constraint.Mqtt_Parsed.NoValue {
			if err == nil {
				Automation.Value = parsed
			} else {
				Automation.Value = string(msg.Payload())
			}
			Automation.Value_Time = time.Now()
		}

	}

	// JSON Object Value
	if Constraint.Mqtt_Parsed.Object != "" {

		jsonMap := make(map[string]interface{})
		json.Unmarshal(msg.Payload(), &jsonMap)
		if jsonMap[Constraint.Mqtt_Parsed.Object] == nil {
			return
		}
		// Set Constraint value
		Constraint.Value = jsonMap[Constraint.Mqtt_Parsed.Object]
		Constraint.Value_Time = time.Now()
		// Don't set Automations value to Constraint value
		if !Constraint.Mqtt_Parsed.NoValue {
			Automation.Value = jsonMap[Constraint.Mqtt_Parsed.Object]
			Automation.Value_Time = time.Now()
		}

	}

	// Reset Timeout Ticker
	if Constraint.Mqtt_Parsed.Timeout > 0 {
		Constraint.Mqtt_Parsed.Timeout_Ticker.Reset(Constraint.Mqtt_Parsed.Timeout)
	}

	// Rules
	if Constraint.Mqtt_Parsed.Comparator == "" && Constraint.Mqtt_Parsed.Timeout == 0 {
		setTriggered(Automation, Constraint, true)
		setTriggered(Automation, Constraint, false)
	} else {
		switch Constraint.Value.(type) {
		case float64:
			switch Constraint.Mqtt_Parsed.Value.(type) {
			case float64:
				v1 := Constraint.Value.(float64)
				v2 := Constraint.Mqtt_Parsed.Value.(float64)
				if Constraint.Mqtt_Parsed.Comparator == "=" && v1 == v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "==" && v1 == v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "!=" && v1 != v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == ">=" && v1 >= v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "<=" && v1 <= v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "<" && v1 < v2 {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == ">" && v1 > v2 {
					setTriggered(Automation, Constraint, true)
				} else {
					setTriggered(Automation, Constraint, false)
				}
			default:
				setTriggered(Automation, Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
			}
		case string:
			switch Constraint.Mqtt_Parsed.Value.(type) {
			case string:
				match, err := regexp.MatchString(Constraint.Mqtt_Parsed.Value.(string), Constraint.Value.(string))
				if err != nil {
					log.Error("[MQTT] Error while comparing constraints value: ", err)
				}
				if Constraint.Mqtt_Parsed.Comparator == "=" && match {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "==" && match {
					setTriggered(Automation, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "!=" && !match {
					setTriggered(Automation, Constraint, true)
				} else {
					setTriggered(Automation, Constraint, false)
				}
			default:
				setTriggered(Automation, Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
			}
		}
	}

	/*fmt.Printf("Constraint.Value:%v\n", Constraint.Value)
	fmt.Printf("Constraint.Value(Type):%T\n", Constraint.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value:%v\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value(Type):%T\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Triggered:%v\n", Constraint.Triggered)*/

}

func setTriggered(Automation *Config.Automation_t, Constraint *Config.Constraint_t, triggered bool) {

	if triggered {

		// Set Last Triggered
		Constraint.Triggered_Time = time.Now()

		// Reset Reset Timer
		if Constraint.Mqtt_Parsed.Reset > 0 {
			Constraint.Mqtt_Parsed.Reset_Timer.Reset(Constraint.Mqtt_Parsed.Reset)
		}

	}

	Constraint.Triggered = triggered

	// CheckTriggered
	Config.CheckTriggered(Automation, Constraint.Mqtt_Parsed.NoTrigger)

}
