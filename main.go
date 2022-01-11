package main

import (
	"time"

	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Cron"
	"MqttCommander/Dashbaord"
	"MqttCommander/Http"
	"MqttCommander/Mqtt"
	"MqttCommander/Rule"

	log "github.com/sirupsen/logrus"
)

func main() {

	// Setup Logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)
	log.Info("[MAIN] Welcome to MqttCommader! By Calvin Köcher 2022 [calvin.koecher@alumni.fh-aachen.de]")

	// Setup
	Config.Begin()
	Config.ReadConfig()
	Automation.Begin()
	Rule.Begin()
	Dashbaord.Begin()
	Mqtt.Begin()
	Cron.Begin()
	Http.Begin()

	// Watch Changes
	go func() {
		for {
			if Automation.Read(Config.Get().ConfigPath) > 0 {
				Automation.Deploy()
				Mqtt.Deploy()
				Cron.Deploy()
				Http.Deploy()
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Forever
	for {
		time.Sleep(time.Second)
	}

}
