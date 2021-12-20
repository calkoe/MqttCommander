package main

import (
	"time"

	"MqttCommander/Config"
	"MqttCommander/Cron"
	"MqttCommander/Dashbaord"
	"MqttCommander/Http"
	"MqttCommander/Mqtt"

	log "github.com/sirupsen/logrus"
)

func main() {

	// Setup Logger
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(log.DebugLevel)
	log.Info("[MAIN] Welcome to MqttCommader!")

	// Setup
	Config.SetupConfig()
	Config.ReadConfig()
	Mqtt.Init()
	Dashbaord.Init()

	// Watch Changes
	go func() {
		for {
			if Config.ReadAutomations() > 0 {
				Config.Deploy()
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
