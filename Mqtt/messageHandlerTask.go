package Mqtt

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Rule"
	"regexp"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

func MessageHandlerTask(t time.Time, client MQTT.Client, msg MQTT.Message) {

	// Find matching topics
	for _, rule := range Rule.GetAllByTag("constraint/mqtt") {
		if rule.Initialized && rule.Module.(Mqtt_Parsed_t).Topic == msg.Topic() {
			Automation.RTTstart(rule.AutomationId, t)
			onMessage(rule, client, msg)
		}
	}

}

func onMessage(rule Rule.Rule_t, client MQTT.Client, msg MQTT.Message) {

	// Debug
	// fmt.Printf("%s\n", msg.Payload())

	module := rule.Module.(Mqtt_Parsed_t)

	// Block Retained
	if module.Retained && msg.Retained() {
		return
	}

	var value interface{}

	// Raw Value OR JSON
	if module.Object == "" {
		value = Config.ParseType(string(msg.Payload()))
	} else {
		// Parse JSON Object-Path
		v := gjson.Get(string(msg.Payload()), module.Object)
		if !v.Exists() {
			return
		}
		value = Config.ParseType(v.String())
	}

	// Set Rule value
	Rule.SetValue(rule.Id, value)

	// Set Automation Value
	if !module.NoValue {
		Automation.SetValue(rule.AutomationId, value)
	}

	// Reset Timeout Ticker
	if module.Timeout > 0 {
		module.Timeout_Ticker.Reset(module.Timeout)
	}

	// Comparators
	var err string
	if module.Comparator == "" {
		if module.Timeout == 0 {
			SetTrigger(rule.Id, true)
			if module.Reset == 0 {
				SetTrigger(rule.Id, false)
			}
		} else {
			SetTrigger(rule.Id, false)
		}
	} else {
		switch value.(type) {
		case bool:
			switch module.Value.(type) {
			case bool:
				v1 := value.(bool)
				v2 := module.Value.(bool)
				if module.Comparator == "=" && v1 == v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "==" && v1 == v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "!=" && v1 != v2 {
					SetTrigger(rule.Id, true)
				} else {
					SetTrigger(rule.Id, false)
				}
			default:
				SetTrigger(rule.Id, module.Comparator == "!=")
				err = "Datatype missmatch"
			}
		case float64:
			switch module.Value.(type) {
			case float64:
				v1 := value.(float64)
				v2 := module.Value.(float64)
				if module.Comparator == "=" && v1 == v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "==" && v1 == v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "!=" && v1 != v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == ">=" && v1 >= v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "<=" && v1 <= v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "<" && v1 < v2 {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == ">" && v1 > v2 {
					SetTrigger(rule.Id, true)
				} else {
					SetTrigger(rule.Id, false)
				}
			default:
				SetTrigger(rule.Id, module.Comparator == "!=")
				err = "Datatype missmatch"
			}
		case string:
			switch module.Value.(type) {
			case string:
				match, e := regexp.MatchString(module.Value.(string), value.(string))
				if e != nil {
					log.Error("[MQTT] Error while comparing constraints value: ", err)
				}
				if module.Comparator == "=" && match {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "==" && match {
					SetTrigger(rule.Id, true)
				} else if module.Comparator == "!=" && !match {
					SetTrigger(rule.Id, true)
				} else {
					SetTrigger(rule.Id, false)
				}
			default:
				SetTrigger(rule.Id, module.Comparator == "!=")
				err = "Datatype missmatch"
			}
		default:
			SetTrigger(rule.Id, module.Comparator == "!=")
			err = "Datatype missmatch"
		}

	}

	// Set Error
	Rule.SetError(rule.Id, err)

	/*fmt.Printf("Constraint.Value:%rule\n", Constraint.Value)
	fmt.Printf("Constraint.Value(Type):%T\n", Constraint.Value)
	fmt.Printf("Constraint.Module.Value:%rule\n", Constraint.Module.Value)
	fmt.Printf("Constraint.Module.Value(Type):%T\n", Constraint.Module.Value)
	fmt.Printf("Constraint.Triggered:%rule\n", Constraint.Triggered)*/

}
