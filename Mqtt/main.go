package Mqtt

import (
	"MqttCommander/Config"
	"encoding/json"
	"fmt"
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

var Constraint_regxp = regexp.MustCompile(`\s*(?P<Topic>[[:alnum:]/#+-_]*)(?:\|(?P<Object>[[:alnum:]-_]*))?(?:\s+(?P<Comparator>[<|>|=|!]+))?(?:\s+"(?P<Value_String>.*)")?(?:\s+(?P<Value_Float>[[:digit:]]+\.?,?[[:digit:]]*))?(?:\s+-Reset\s+(?P<Reset>[[:digit:]nsusmssmh]+))?(?:\s+-Timeout\s+(?P<Timeout>[[:digit:]nsusmssmh]+))?(?:\s+-BlockRetained\s+(?P<Retained>[01]))?`)
var Action_regxp = regexp.MustCompile(`\s*(?P<Topic>[[:alnum:]/#+-_]*)(?:\|(?P<Object>[[:alnum:]-_]*))?(?:\s+(?P<Comparator>[=]+))?(?:\s+"(?P<Value_String>.*)")?(?:\s+(?P<Value_Float>[[:digit:]]+\.?,?[[:digit:]]*))?(?:\s+-Retained\s+(?P<Retained>[01]))?`)

//define a function for the default message handler
var DefaultPublishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	//fmt.Printf("TOPIC: %s\n", msg.Topic())
	//fmt.Printf("MSG: %s\n", msg.Payload())

	// Find matching topics
	for Automation_k := range Config.Config.Automations {
		Automation := &Config.Config.Automations[Automation_k]

		// Find matching topics
		var triggeredChanged bool
		for Constraint_k := range Automation.Constraints {
			Constraint := &Automation.Constraints[Constraint_k]
			if Constraint.Mqtt != "" && Constraint.Mqtt_Parsed.Topic == msg.Topic() {
				if onMessage(Automation, Constraint, client, msg) {
					triggeredChanged = true
				}
			}
		}

		// CheckTriggered if valueChanged
		if triggeredChanged {
			Config.CheckTriggered(Automation)
		}

	}
}

func Connect() {

	log.Infof("[MQTT] Connecting to MQTT Server: %s", Config.Config.System.MqttUri)

	// Create Client
	opts := MQTT.NewClientOptions()
	opts.AddBroker(Config.Config.System.MqttUri)
	opts.SetClientID(Config.Config.System.Name)

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

				match := Constraint_regxp.FindStringSubmatch(Constraint.Mqtt)

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

				if len(match) == 9 {

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
					Constraint.Mqtt_Parsed.Reset, _ = time.ParseDuration(match[6])
					Constraint.Mqtt_Parsed.Timeout, _ = time.ParseDuration(match[7])
					Constraint.Mqtt_Parsed.BlockRetained, _ = strconv.ParseBool(match[8])

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
								setTriggered(Constraint_c, false)
								Config.CheckTriggered(Automation_c)
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
								setTriggered(Constraint_c, true)
								Config.CheckTriggered(Automation_c)
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

				match := Action_regxp.FindStringSubmatch(Action.Mqtt)

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

				if len(match) == 7 {

					Action.Mqtt_Parsed.Topic = match[1]
					Action.Mqtt_Parsed.Object = match[2]

					if Action.Mqtt_Parsed.Object != "" {
						// Value_String
						if match[4] != "" {
							Action.Mqtt_Parsed.Value = fmt.Sprintf("{\"%s\":\"%s\"}", Action.Mqtt_Parsed.Object, match[4])
						}
						// Value_Float
						if match[5] != "" {
							Action.Mqtt_Parsed.Value = fmt.Sprintf("{\"%s\":%s}", Action.Mqtt_Parsed.Object, match[5])
						}
					} else {
						// V alue_String
						if match[4] != "" {
							Action.Mqtt_Parsed.Value = match[4]

						}
						// Value_Float
						if match[5] != "" {
							Action.Mqtt_Parsed.Value = match[5]
						}
					}

					Action.Mqtt_Parsed.Retained, _ = strconv.ParseBool(match[6])

					// Setup Trigger Handler
					Action.Trigger = func() {
						Action_c := Action
						Client.Publish(Action_c.Mqtt_Parsed.Topic, 2, Action_c.Mqtt_Parsed.Retained, Action_c.Mqtt_Parsed.Value)
					}

				}

			}
		}

	}

	log.Info("[MQTT] Initializiation completed!")

}

func onMessage(Automation *Config.Automation_t, Constraint *Config.Constraint_t, client MQTT.Client, msg MQTT.Message) bool {

	// Debug
	// fmt.Printf("%s\n", msg.Payload())

	// Block Retained
	if Constraint.Mqtt_Parsed.BlockRetained && msg.Retained() {
		return false
	}

	// Reset Timeout Ticker
	if Constraint.Mqtt_Parsed.Timeout > 0 {
		Constraint.Mqtt_Parsed.Timeout_Ticker.Reset(Constraint.Mqtt_Parsed.Timeout)
	}

	// Save old value
	ConstraintValueOld := Constraint.Value
	ConstraintTriggeredOld := Constraint.Triggered

	// Value = Raw
	if Constraint.Mqtt_Parsed.Object == "" {
		parsed, err := strconv.ParseFloat(string(msg.Payload()), 64)
		if err == nil {
			Constraint.Value = parsed
			Automation.Value = parsed
		} else {
			Constraint.Value = string(msg.Payload())
			Automation.Value = string(msg.Payload())
		}
		Constraint.Value_Time = time.Now()
		Automation.Value_Time = time.Now()
	}

	// Value = JSON Object
	if Constraint.Mqtt_Parsed.Object != "" {
		jsonMap := make(map[string]interface{})
		json.Unmarshal(msg.Payload(), &jsonMap)
		if jsonMap[Constraint.Mqtt_Parsed.Object] == nil {
			return false
		}
		Constraint.Value = jsonMap[Constraint.Mqtt_Parsed.Object]
		Automation.Value = jsonMap[Constraint.Mqtt_Parsed.Object]
		Constraint.Value_Time = time.Now()
		Automation.Value_Time = time.Now()
	}

	// Return if no changes
	if ConstraintValueOld == Constraint.Value {
		return false
	}

	// Comparator
	switch Constraint.Value.(type) {
	case float64:
		switch Constraint.Mqtt_Parsed.Value.(type) {
		case float64:
			v1 := Constraint.Value.(float64)
			v2 := Constraint.Mqtt_Parsed.Value.(float64)
			if Constraint.Mqtt_Parsed.Comparator == "=" && v1 == v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "==" && v1 == v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "!=" && v1 != v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == ">=" && v1 >= v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "<=" && v1 <= v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "<" && v1 < v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == ">" && v1 > v2 {
				setTriggered(Constraint, true)
			} else {
				setTriggered(Constraint, false)
			}
		default:
			setTriggered(Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
		}
	case string:
		switch Constraint.Mqtt_Parsed.Value.(type) {
		case string:
			v1 := Constraint.Value.(string)
			v2 := Constraint.Mqtt_Parsed.Value.(string)
			if Constraint.Mqtt_Parsed.Comparator == "=" && v1 == v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "==" && v1 == v2 {
				setTriggered(Constraint, true)
			} else if Constraint.Mqtt_Parsed.Comparator == "!=" && v1 != v2 {
				setTriggered(Constraint, true)
			} else {
				setTriggered(Constraint, false)
			}
		default:
			setTriggered(Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
		}
	}

	// return if triggered changed
	return ConstraintTriggeredOld != Constraint.Triggered

	/*fmt.Printf("Constraint.Value:%v\n", Constraint.Value)
	fmt.Printf("Constraint.Value(Type):%T\n", Constraint.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value:%v\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value(Type):%T\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Triggered:%v\n", Constraint.Triggered)*/

}

func setTriggered(Constraint *Config.Constraint_t, triggered bool) {

	if triggered {

		// Set Last Triggered
		Constraint.Triggered_Time = time.Now()

		// Reset Reset Timer
		if Constraint.Mqtt_Parsed.Reset > 0 {
			Constraint.Mqtt_Parsed.Reset_Timer.Reset(Constraint.Mqtt_Parsed.Reset)
		}

	}

	Constraint.Triggered = triggered

}
