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

var Client MQTT.Client
var Expression *regexp.Regexp

//define a function for the default message handler
var DefaultPublishHandler MQTT.MessageHandler = func(client MQTT.Client, msg MQTT.Message) {
	//fmt.Printf("TOPIC: %s\n", msg.Topic())
	//fmt.Printf("MSG: %s\n", msg.Payload())

	// Start RTT Measurement
	go func(t time.Time) {

		// Find matching topics
		for id, AutomationCopy := range Config.CopyAutomations() {
			for Constraint_k := range AutomationCopy.Constraints {

				ConstraintCopy := Config.CopyConstraint(&AutomationCopy.Constraints[Constraint_k])
				if ConstraintCopy.Mqtt != "" && ConstraintCopy.Mqtt_Parsed.Topic == msg.Topic() {
					Config.RTTstart(id, t)
					OnMessage(id, &AutomationCopy.Constraints[Constraint_k], client, msg)
				}

			}
		}

	}(time.Now())

}

func Init() {

	ConfigFileCopy := Config.CopyConfigFile()

	Expression = regexp.MustCompile(`^\s*(?P<Topic>[^\.\s]*)(?:\.(?P<Object>\S*))?(?:\s+(?P<Comparator>[^-][\S]*))?(?:\s+"(?P<Value_String>.*)")?(?:\s+(?P<Value_Float>(?:-[0-9.]+)|(?:[^-][\S]*)))?.*$`)

	log.Infof("[MQTT] Connecting to MQTT Server: %s", Config.CopyConfigFile().MqttUri)

	// Create Client
	opts := MQTT.NewClientOptions()
	opts.AddBroker(ConfigFileCopy.MqttUri)
	opts.SetClientID(ConfigFileCopy.Name)

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
		for _, AutomationCopy := range Config.CopyAutomations() {
			for Constraint_k := range AutomationCopy.Constraints {
				ConstraintCopy := Config.CopyConstraint(&AutomationCopy.Constraints[Constraint_k])
				if ConstraintCopy.Mqtt != "" && ConstraintCopy.Initialized {
					ConstraintCopy.Mqtt_Parsed.Token = Client.Subscribe(ConstraintCopy.Mqtt_Parsed.Topic, 2, nil)
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
	Client.Connect()
	/*token := Client.Connect()
	token.Wait()
	if token.Error() != nil {
		log.Errorf("[MQTT] Connection to MQTT server failed, error: %s", token.Error())
	}*/

}

// Init Constraints and Actions
func Deploy() {

	for id, AutomationCopy := range Config.CopyAutomations() {

		// Setup Constraints
		for Constraint_k := range AutomationCopy.Constraints {

			ConstraintCopy := Config.CopyConstraint(&AutomationCopy.Constraints[Constraint_k])
			if ConstraintCopy.Mqtt != "" && !ConstraintCopy.Initialized {

				AutomationCopy.Constraints[Constraint_k].Mutex.Lock()
				Constraint := &AutomationCopy.Constraints[Constraint_k]

				Constraint.Initialized = true

				match := Expression.FindStringSubmatch(Constraint.Mqtt)

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
						go func(id int, Constraint *Config.Constraint_t) {
							_, ok := Config.CopyAutomation(id)
							for ok {
								<-Constraint.Mqtt_Parsed.Reset_Timer.C
								_, ok := Config.CopyAutomation(id)
								if ok {
									Constraint.Mutex.Lock()
									setTriggered(id, Constraint, false)
									Constraint.Mutex.Unlock()
								} else {
									break
								}
							}
						}(id, Constraint)
					}

					// Add Timeout Ticker
					if Constraint.Mqtt_Parsed.Timeout > 0 {
						Constraint.Mqtt_Parsed.Timeout_Ticker = time.NewTicker(Constraint.Mqtt_Parsed.Timeout)
						go func(id int, Constraint *Config.Constraint_t) {
							_, ok := Config.CopyAutomation(id)
							for ok {
								<-Constraint.Mqtt_Parsed.Timeout_Ticker.C
								_, ok := Config.CopyAutomation(id)
								if ok {
									Constraint.Mutex.Lock()
									setTriggered(id, Constraint, true)
									Constraint.Mutex.Unlock()
								} else {
									break
								}
							}
						}(id, Constraint)
					}

				}

				AutomationCopy.Constraints[Constraint_k].Mutex.Unlock()

			}

		}

		// Setup Actions
		for Action_k := range AutomationCopy.Actions {

			ActionCopy := Config.CopyAction(&AutomationCopy.Actions[Action_k])
			if ActionCopy.Mqtt != "" && !ActionCopy.Initialized {

				AutomationCopy.Actions[Action_k].Mutex.Lock()
				Action := &AutomationCopy.Actions[Action_k]

				Action.Initialized = true

				match := Expression.FindStringSubmatch(Action.Mqtt)

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
					Action.Mqtt_Parsed.Reverse, _ = strconv.ParseBool(Config.Find(`-Reverse\s+(\S+)`, Action.Mqtt))

					// Parse Template
					var err error
					Action.Mqtt_Parsed.Template, err = template.New("value").Parse(Action.Mqtt_Parsed.Value)
					if err != nil {
						log.Errorf("[MQTT] error while parsing Template: %s", err)
						return
					}

					// Setup Trigger Handler
					Action.Trigger = func(AutomationCopy Config.Automation_t, ActionCopy Config.Action_t) {

						// Run Action
						if ActionCopy.Triggered != ActionCopy.Mqtt_Parsed.Reverse && ActionCopy.Mqtt_Parsed.Template != nil {
							var payload string
							var buf bytes.Buffer
							ActionCopy.Mqtt_Parsed.Template.Execute(&buf, AutomationCopy)
							if ActionCopy.Mqtt_Parsed.Object != "" {
								if ActionCopy.Mqtt_Parsed.IsString {
									payload = fmt.Sprintf("{\"%s\":\"%s\"}", ActionCopy.Mqtt_Parsed.Object, buf.String())
								} else {
									payload = fmt.Sprintf("{\"%s\":%s}", ActionCopy.Mqtt_Parsed.Object, buf.String())
								}
							} else {
								payload = buf.String()
							}
							Client.Publish(ActionCopy.Mqtt_Parsed.Topic, 2, ActionCopy.Mqtt_Parsed.Retained, payload)
						}

						// Stop RTT Measurement
						Config.RTTstop(AutomationCopy.Id)

					}
				}

				AutomationCopy.Actions[Action_k].Mutex.Unlock()

			}

		}

	}

}

func OnMessage(id int, Constraint *Config.Constraint_t, client MQTT.Client, msg MQTT.Message) {

	// Debug
	// fmt.Printf("%s\n", msg.Payload())

	Constraint.Mutex.Lock()
	defer Constraint.Mutex.Unlock()

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
				Config.SetValue(id, parsed)
			} else {
				Config.SetValue(id, string(msg.Payload()))
			}
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
			Config.SetValue(id, jsonMap[Constraint.Mqtt_Parsed.Object])
		}

	}

	// Reset Timeout Ticker
	if Constraint.Mqtt_Parsed.Timeout > 0 {
		Constraint.Mqtt_Parsed.Timeout_Ticker.Reset(Constraint.Mqtt_Parsed.Timeout)
	}

	// Rules
	if Constraint.Mqtt_Parsed.Comparator == "" && Constraint.Mqtt_Parsed.Timeout == 0 {
		setTriggered(id, Constraint, true)
		setTriggered(id, Constraint, false)
	} else {
		switch Constraint.Value.(type) {
		case float64:
			switch Constraint.Mqtt_Parsed.Value.(type) {
			case float64:
				v1 := Constraint.Value.(float64)
				v2 := Constraint.Mqtt_Parsed.Value.(float64)
				if Constraint.Mqtt_Parsed.Comparator == "=" && v1 == v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "==" && v1 == v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "!=" && v1 != v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == ">=" && v1 >= v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "<=" && v1 <= v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "<" && v1 < v2 {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == ">" && v1 > v2 {
					setTriggered(id, Constraint, true)
				} else {
					setTriggered(id, Constraint, false)
				}
			default:
				setTriggered(id, Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
			}
		case string:
			switch Constraint.Mqtt_Parsed.Value.(type) {
			case string:
				match, err := regexp.MatchString(Constraint.Mqtt_Parsed.Value.(string), Constraint.Value.(string))
				if err != nil {
					log.Error("[MQTT] Error while comparing constraints value: ", err)
				}
				if Constraint.Mqtt_Parsed.Comparator == "=" && match {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "==" && match {
					setTriggered(id, Constraint, true)
				} else if Constraint.Mqtt_Parsed.Comparator == "!=" && !match {
					setTriggered(id, Constraint, true)
				} else {
					setTriggered(id, Constraint, false)
				}
			default:
				setTriggered(id, Constraint, Constraint.Mqtt_Parsed.Comparator == "!=")
			}
		}
	}

	/*fmt.Printf("Constraint.Value:%v\n", Constraint.Value)
	fmt.Printf("Constraint.Value(Type):%T\n", Constraint.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value:%v\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Mqtt_Parsed.Value(Type):%T\n", Constraint.Mqtt_Parsed.Value)
	fmt.Printf("Constraint.Triggered:%v\n", Constraint.Triggered)*/

}

func setTriggered(id int, Constraint *Config.Constraint_t, triggered bool) {

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
	NoTrigger := Constraint.Mqtt_Parsed.NoTrigger
	Constraint.Mutex.Unlock()
	Config.CheckTriggered(id, NoTrigger)
	Constraint.Mutex.Lock()

}
