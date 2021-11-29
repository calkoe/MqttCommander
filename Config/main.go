package Config

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creasty/defaults"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Map = sync.Map

type ConfigFile_t struct {
	Name             string `yaml:"Name"`
	Timezone         string `yaml:"Timezone"`
	Timezone_parsed  *time.Location
	MqttUri          string `yaml:"MqttUri"`
	AutomationsPath  string `yaml:"AutomationsPath"`
	AutomationsFiles map[string]string
}
type Automation_t struct {
	Id               uint64
	Name             string `yaml:"Name"`
	Mode             string `default:"AND" yaml:"Mode"`
	Retrigger        bool   `yaml:"Retrigger"`
	Hidden           bool   `yaml:"Hidden"`
	Retrigger_Active bool
	Pause            time.Duration `yaml:"Pause"`
	Reminder         time.Duration `yaml:"Reminder"`
	Reminder_Ticker  *time.Ticker
	Reminder_Active  bool
	Delay            time.Duration `yaml:"Delay"`
	Delay_Timer      *time.Timer
	Delay_Active     bool
	Constraints      []Constraint_t `yaml:"Constraints"`
	Actions          []Action_t     `yaml:"Actions"`
	Triggered        bool           `yaml:"Triggered"`
	File             string
	Triggered_Time   time.Time
	Value            interface{}
	Value_Time       time.Time
	Initialized      bool
}

type Constraint_t struct {
	Mqtt        string `yaml:"Mqtt"`
	Mqtt_Parsed struct {
		Topic          string
		Object         string
		Comparator     string
		Value          interface{}
		BlockRetained  bool
		Reset          time.Duration
		Reset_Timer    *time.Timer
		Timeout        time.Duration
		Timeout_Ticker *time.Ticker
		NoTrigger      bool
		NoValue        bool
		Token          MQTT.Token
	}
	Cron        string `yaml:"Cron"`
	Cron_Parsed struct {
		Expression  *cronexpr.Expression
		NextTime    time.Time
		Cron_Timer  *time.Timer
		Reset       time.Duration
		Reset_Timer *time.Timer
		NoTrigger   bool
	}
	Triggered      bool
	Triggered_Time time.Time
	Value          interface{}
	Value_Time     time.Time
	Initialized    bool
	Mutex          sync.RWMutex
}

type Action_t struct {
	Mqtt        string `yaml:"Mqtt"`
	Mqtt_Parsed struct {
		Topic    string
		Object   string
		Value    string
		IsString bool
		Retained bool
	}
	Http           string `yaml:"Http"`
	Trigger        func(Automation_t, *Action_t)
	Triggered      bool
	Triggered_Time time.Time
	Initialized    bool
	Mutex          sync.RWMutex
}

var IdCounter uint64
var ConfigPath string

var configFile ConfigFile_t
var configFileMutex sync.RWMutex

var automations map[uint64]*Automation_t
var automationsMutex sync.RWMutex

//go:embed config.yml
var config_yml []byte

//go:embed automations/automation.yml
var automation_yml []byte

func SetupConfig() {

	// Set ConfigPath
	if os.Getenv("MQTTC_CONFIG_PATH") == "" {

		// Automatic
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		ConfigPath = filepath.Dir(ex) + "/Config"
		log.Infof("[CONFIG] ConfigPath has been automatically set to: %s", ConfigPath)

	} else {

		//Manual
		ConfigPath = os.Getenv("MQTTC_CONFIG_PATH")
		log.Infof("[CONFIG] ConfigPath has been manually set to: %s", ConfigPath)

	}

	// Create configFile.AutomationsFiles
	configFile.AutomationsFiles = make(map[string]string)

	// Create automations
	automations = make(map[uint64]*Automation_t)

}

func ReadConfig() {

	configFileMutex.Lock()
	defer configFileMutex.Unlock()

	// Check ConfigPath
	if _, err := os.Stat(ConfigPath); err != nil {
		if os.IsNotExist(err) {
			log.Warn("[CONFIG] " + ConfigPath + " does not exits, creating it!")
			os.Mkdir(ConfigPath, 0744)
			log.Warn("[CONFIG] " + ConfigPath + "/config.yml does not exits, creating it!")
			os.WriteFile(ConfigPath+"/config.yml", config_yml, 0644)
		}
	}

	// Read Main Config File (without automations!)
	yamlFile, err := ioutil.ReadFile(ConfigPath + "/config.yml")
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(yamlFile, &configFile)
	log.Infof("[CONFIG] Successfully read main config file: %s", ConfigPath+"/config.yml")

	// Check ConfigPath/automations
	configFile.AutomationsPath = ConfigPath + "/automations"
	log.Infof("[CONFIG] Set AutomationsPath to: %s", configFile.AutomationsPath)
	if _, err := os.Stat(configFile.AutomationsPath); err != nil {
		if os.IsNotExist(err) {
			log.Warnf("[CONFIG] %S does not exits, creating it!", configFile.AutomationsPath)
			os.Mkdir(configFile.AutomationsPath, 0744)
			log.Warnf("[CONFIG] %s/automation.yml does not exits, creating it!", configFile.AutomationsPath)
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

}

func ReadAutomations() uint {

	automationsMutex.Lock()
	defer automationsMutex.Unlock()

	configFileMutex.Lock()
	defer configFileMutex.Unlock()

	var affectedFiles uint

	// List automations Files
	FilesInFolder := make(map[string]string)
	err := filepath.Walk(configFile.AutomationsPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".yml" {
			file, err := os.Open(path)
			if err != nil {
				log.Errorf("Problem opening File %s", err)
			}
			hash := sha1.New()
			io.Copy(hash, file)
			FilesInFolder[path] = hex.EncodeToString(hash.Sum(nil))
			file.Close()
		}
		return nil
	})
	if err != nil {
		log.Errorf("[CONFIG] walk error [%v]\n", err)
	}

	// Check if files were removed
	for path := range configFile.AutomationsFiles {
		if _, found := FilesInFolder[path]; !found {

			// Deleting corresponding automations
			deleteByFile(path)

			// Delete File from Config Files
			delete(configFile.AutomationsFiles, path)

			// Log
			log.Infof("[CONFIG] A file has been removed, corresponding automations deleted! Path: %s", path)

			// Count
			affectedFiles++

		}
	}

	// Load new or changed Files
	for path := range FilesInFolder {
		if _, found := configFile.AutomationsFiles[path]; !found || configFile.AutomationsFiles[path] != FilesInFolder[path] {

			// Deleting corresponding automations
			deleteByFile(path)

			// Read automations
			FileContent, _ := ioutil.ReadFile(path)
			var automationsNew []Automation_t
			yaml.Unmarshal(FileContent, &automationsNew)
			for idNew := range automationsNew {
				automationsNew[idNew].Id = IdCounter
				automationsNew[idNew].File = path
				automations[IdCounter] = &automationsNew[idNew]
				IdCounter++
			}

			// Add File to configFile.Files
			configFile.AutomationsFiles[path] = FilesInFolder[path]

			// Log
			log.Infof("[CONFIG] read %d automations from file: %s [Hash: %s]", len(automationsNew), path, FilesInFolder[path])

			// Count
			affectedFiles++

		}
	}

	return affectedFiles

}

func Deploy() {

	automationsMutex.Lock()
	defer automationsMutex.Unlock()

	// Setup automations
	for id := range automations {

		Automation := automations[id]

		if !Automation.Initialized {
			Automation.Initialized = true

			// Log
			log.WithFields(log.Fields{
				"Id":          id,
				"Name":        Automation.Name,
				"File":        strings.Replace(Automation.File, configFile.AutomationsPath+"/", "", 1),
				"Constraints": len(Automation.Constraints),
				"Actions":     len(Automation.Actions),
			}).Debugf("[CONFIG] Deploy Automation [%p]", Automation)

			// Add Delay Timer
			if Automation.Delay > 0 {
				Automation.Delay_Timer = time.NewTimer(Automation.Delay)
				Automation.Delay_Timer.Stop()
				go func(id uint64) {
					Automation, ok := GetAutomation(id)
					for ok {
						<-Automation.Delay_Timer.C
						Automation, ok := GetAutomation(id)
						if ok {
							go RunActions(id)
							automationsMutex.Lock()
							Automation.Delay_Active = false
							automationsMutex.Unlock()
						} else {
							break
						}
					}
				}(id)
			}

			// Add Reminder Ticker
			if Automation.Reminder > 0 {
				Automation.Reminder_Ticker = time.NewTicker(Automation.Reminder)
				Automation.Reminder_Ticker.Stop()
				go func(id uint64) {
					Automation, ok := GetAutomation(id)
					for ok {
						<-Automation.Reminder_Ticker.C
						_, ok := GetAutomation(id)
						if ok {
							go RunActions(id)
						} else {
							break
						}
					}
				}(id)
			}

			// If inital triggered
			if Automation.Triggered {
				Automation.Triggered = false
				setTriggered(Automation, true)
			}

		}
	}
}

func CheckTriggered(id uint64, NoTrigger bool) {

	automationsMutex.Lock()
	defer automationsMutex.Unlock()
	Automation := automations[id]

	// Debug
	// log.Debugf("CheckTriggered Automation: %s", Automation.Name)

	triggered := 0

	for Constraint_k := range Automation.Constraints {
		Automation.Constraints[Constraint_k].Mutex.Lock()
		defer Automation.Constraints[Constraint_k].Mutex.Unlock()
		if Automation.Constraints[Constraint_k].Triggered {
			triggered++
		}
	}

	if Automation.Mode == "AND" && triggered == len(Automation.Constraints) {
		if !NoTrigger {
			setTriggered(Automation, true)
		}
	} else if Automation.Mode == "OR" && triggered >= 1 {
		if !NoTrigger {
			setTriggered(Automation, true)
		}
	} else {
		setTriggered(Automation, false)
	}
}

func setTriggered(Automation *Automation_t, triggered bool) {

	// Debug
	// log.Debugf("SetTriggered automations: %s, Automation.triggered: %t, triggered: %t", automations[id].Name, Automation.Triggered, triggered)

	if !triggered {

		// Retrigger
		Automation.Retrigger_Active = false

		// Setting: Delay
		if Automation.Delay > 0 {
			Automation.Delay_Timer.Stop()
			Automation.Delay_Active = false
		}

		// Setting: Reminder
		if Automation.Reminder > 0 {
			Automation.Reminder_Ticker.Stop()
			Automation.Reminder_Active = false
		}

		//stopActions
		go StopActions(Automation.Id)

	}

	if triggered {

		// Retrigger
		if Automation.Triggered {
			if Automation.Retrigger {
				Automation.Retrigger_Active = true
			} else {
				return
			}
		}

		// Setting: Pause
		if time.Since(Automation.Triggered_Time) < Automation.Pause {
			return
		}
		Automation.Triggered_Time = time.Now()

		// Setting: Delay
		if Automation.Delay > 0 {
			Automation.Delay_Timer.Reset(Automation.Delay)
			Automation.Delay_Active = true
		} else {
			go RunActions(Automation.Id)
		}

		// Setting Reminder
		if Automation.Reminder > 0 {
			Automation.Reminder_Ticker.Reset(Automation.Reminder)
			Automation.Reminder_Active = true
		}

	}

	//	Log
	if Automation.Triggered != triggered {
		log.WithFields(log.Fields{
			"Id":        Automation.Id,
			"Name":      Automation.Name,
			"File":      strings.Replace(Automation.File, configFile.AutomationsPath+"/", "", 1),
			"Value":     Automation.Value,
			"Triggered": triggered,
		}).Debugf("[CONFIG] Automation changed [%p]", Automation)
	}

	Automation.Triggered = triggered
}

func RunActions(id uint64) {

	Automation, ok := GetAutomation(id)
	if !ok {
		return
	}

	//log.Errorf("RUN ACTIONS ID=%d [%p]!", Automation.Id, Automation)
	for Action_k := range Automation.Actions {
		Automation.Actions[Action_k].Mutex.Lock()
		defer Automation.Actions[Action_k].Mutex.Unlock()
		Action := &Automation.Actions[Action_k]
		if Automation.Actions[Action_k].Trigger != nil {
			go Automation.Actions[Action_k].Trigger(Automation, Action)
			Automation.Actions[Action_k].Triggered = true
			Automation.Actions[Action_k].Triggered_Time = time.Now()
		}
	}

}

func StopActions(id uint64) {

	Automation, ok := GetAutomation(id)
	if !ok {
		return
	}

	for Action_k := range Automation.Actions {
		Automation.Actions[Action_k].Mutex.Lock()
		defer Automation.Actions[Action_k].Mutex.Unlock()
		Automation.Actions[Action_k].Triggered = false
	}

}

func SetValue(id uint64, value interface{}) {

	automationsMutex.Lock()
	defer automationsMutex.Unlock()
	Automation := automations[id]
	Automation.Value = value
	Automation.Value_Time = time.Now()

}

// Setup Automation_t Defaults
func (s *Automation_t) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaults.Set(s)
	type plain Automation_t
	if err := unmarshal((*plain)(s)); err != nil {
		return err
	}

	return nil
}

func GetAutomation(id uint64) (Automation_t, bool) {
	automationsMutex.RLock()
	defer automationsMutex.RUnlock()
	a, ok := automations[id]
	if ok {
		return *a, true
	} else {
		return Automation_t{}, false
	}
}

func GetAutomations() map[uint64]Automation_t {

	automationsMutex.RLock()
	defer automationsMutex.RUnlock()
	ret := make(map[uint64]Automation_t)
	for id := range automations {
		ret[id] = *automations[id]
	}
	return ret

}

func GetConfigFile() ConfigFile_t {

	configFileMutex.Lock()
	defer configFileMutex.Unlock()
	return configFile

}

func deleteByFile(file string) {

	// Delete existing automations by File
	for id := range automations {
		if automations[id].File == file {
			log.Debugf("[CONFIG] Delete Automation! [%p]", automations[id])
			delete(automations, id)
		}
	}

}

func Find(search string, data string) (match string) {
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
