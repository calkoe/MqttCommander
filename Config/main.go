package Config

import (
	_ "embed"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

//go:embed config.yml
var config_yml []byte

//go:embed Automations/automation.yml
var automation_yml []byte

type ConfigFile_t struct {
	Name            string `yaml:"Name"`
	Timezone        string `yaml:"Timezone"`
	Timezone_parsed *time.Location
	MqttUri         string `yaml:"MqttUri"`
	MqttQos         byte   `yaml:"MqttQos"`
	AutomationsPath string `yaml:"AutomationsPath"`
	Muted           bool   `yaml:"Muted"`
	ConfigPath      string
}

var configFile ConfigFile_t

//var mutex deadlock.RWMutex
var mutex sync.RWMutex

// PUBLIC //

func Begin() {

	mutex.Lock()
	defer mutex.Unlock()

	// Set ConfigPath
	if os.Getenv("MQTTC_CONFIG_PATH") == "" {

		// Automatic
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		configFile.ConfigPath = filepath.Dir(ex) + "/Config"
		log.Infof("[CONFIG] ConfigPath has been automatically set to: %s", configFile.ConfigPath)

	} else {

		//Manual
		configFile.ConfigPath = os.Getenv("MQTTC_CONFIG_PATH")
		log.Infof("[CONFIG] ConfigPath has been manually set to: %s", configFile.ConfigPath)

	}

}

func ReadConfig() {

	mutex.Lock()
	defer mutex.Unlock()

	// Create ConfigPath
	if _, err := os.Stat(configFile.ConfigPath); err != nil {
		if os.IsNotExist(err) {
			log.Infof("[CONFIG] " + configFile.ConfigPath + " does not exits, creating it!")
			os.Mkdir(configFile.ConfigPath, 0744)
		}
	}

	// Create ConfigPath/config.yml
	if _, err := os.Stat(configFile.ConfigPath + "/config.yml"); err != nil {
		if os.IsNotExist(err) {
			log.Infof("[CONFIG] " + configFile.ConfigPath + "/config.yml does not exits, creating it!")
			os.WriteFile(configFile.ConfigPath+"/config.yml", config_yml, 0644)
		}
	}

	// Read Main Config File (without automations!)
	yamlFile, err := ioutil.ReadFile(configFile.ConfigPath + "/config.yml")
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(yamlFile, &configFile)
	log.Infof("[CONFIG] Successfully read main config file: %s", configFile.ConfigPath+"/config.yml")

	// Create ConfigPath/automations
	configFile.AutomationsPath = configFile.ConfigPath + "/Automations"
	log.Infof("[CONFIG] Set AutomationsPath to: %s", configFile.AutomationsPath)
	if _, err := os.Stat(configFile.AutomationsPath); err != nil {
		if os.IsNotExist(err) {
			log.Infof("[CONFIG] %s does not exits, creating it!", configFile.AutomationsPath)
			os.Mkdir(configFile.AutomationsPath, 0744)
			log.Infof("[CONFIG] %s does not exits, creating it!", configFile.AutomationsPath+"/automation.yml")
			os.WriteFile(configFile.AutomationsPath+"/automation.yml", automation_yml, 0644)
		}
	}

	// Setup Timezone
	if configFile.Timezone == "" {
		configFile.Timezone = "UTC"
	}
	configFile.Timezone_parsed, err = time.LoadLocation(configFile.Timezone)
	if err != nil {
		log.Errorf("[CONFIG] Error while loading Timezone %s %s", configFile.Timezone, err)
		panic("Timezone-Error!")
	}
	log.Infof("[CONFIG] Using Timezone: %s, Current Time: %s", configFile.Timezone, time.Now().In(configFile.Timezone_parsed))

	// Muted Mode
	if configFile.Muted {
		log.Warn("[CONFIG] Running in Muted Mode!")
	}

}

// HELPER //

func Get() ConfigFile_t {

	mutex.RLock()
	defer mutex.RUnlock()
	return configFile

}

func FindParm(search string, data string) (match string) {
	exp, err := regexp.Compile(search)
	if err != nil {
		log.Errorf("[CONFIG] Error while Parsing Regex: %s", err)
	}
	results := exp.FindStringSubmatch(data)
	if len(results) == 2 {
		match = results[1]
	}
	return
}

func ParseType(input string) interface{} {

	// Trim Text
	input = strings.Trim(input, " ")

	// Is Boolean
	if input == "true" || input == "True" || input == "TRUE" {
		return true
	}
	if input == "false" || input == "False" || input == "FALSE" {
		return false
	}

	// Is float64
	parsed, err := strconv.ParseFloat(input, 64)
	if err == nil {
		return parsed
	}

	// Is String
	input = strings.Trim(input, "\"")
	return input

}
