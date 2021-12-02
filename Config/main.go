package Config

import (
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/creasty/defaults"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorhill/cronexpr"
	"github.com/sasha-s/go-deadlock"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type ConfigFile_t struct {
	Name             string `yaml:"Name"`
	Timezone         string `yaml:"Timezone"`
	Timezone_parsed  *time.Location
	MqttUri          string `yaml:"MqttUri"`
	AutomationsPath  string `yaml:"AutomationsPath"`
	AutomationsFiles map[string]string
	Muted            bool `yaml:"Muted"`
	IdCounter        int
	ConfigPath       string
}
type Automation_t struct {
	Id               int
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
	RTTstart         time.Time
	RTTduration      time.Duration
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
	Mutex          deadlock.RWMutex
}

type Action_t struct {
	Mqtt        string `yaml:"Mqtt"`
	Mqtt_Parsed struct {
		Topic    string
		Object   string
		Value    string
		IsString bool
		Retained bool
		Reverse  bool
		Template *template.Template
	}
	Http        string `yaml:"Http"`
	Http_Parsed struct {
		Template *template.Template
	}
	Trigger        func(Automation_t, Action_t)
	Triggered      bool
	Triggered_Time time.Time
	Initialized    bool
	Mutex          deadlock.RWMutex
}

var configFile ConfigFile_t
var configFileMutex deadlock.RWMutex

var automations map[int]*Automation_t
var automationsMutex deadlock.RWMutex

//go:embed config.yml
var config_yml []byte

//go:embed Automations/automation.yml
var automation_yml []byte

func SetupConfig() {

	configFileMutex.Lock()
	defer configFileMutex.Unlock()

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

	// Create configFile.AutomationsFiles
	configFile.AutomationsFiles = make(map[string]string)

	// Create automations
	automations = make(map[int]*Automation_t)

}

func ReadConfig() {

	configFileMutex.Lock()
	defer configFileMutex.Unlock()

	// Check ConfigPath
	if _, err := os.Stat(configFile.ConfigPath); err != nil {
		if os.IsNotExist(err) {
			log.Infof("[CONFIG] " + configFile.ConfigPath + " does not exits, creating it!")
			os.Mkdir(configFile.ConfigPath, 0744)
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

	// Check ConfigPath/automations
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

func ReadAutomations() uint {

	ConfigFileCopy := CopyConfigFile()

	FilesInFolder := make(map[string]string)
	var affectedFiles uint

	// List automations Files
	err := filepath.Walk(ConfigFileCopy.AutomationsPath, func(path string, info os.FileInfo, err error) error {
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
	for path := range ConfigFileCopy.AutomationsFiles {
		if _, found := FilesInFolder[path]; !found {

			// Deleting corresponding automations
			DeleteAutomation(path)

			// Delete File from Config Files
			configFileMutex.Lock()
			delete(configFile.AutomationsFiles, path)
			configFileMutex.Unlock()

			// Log
			log.Infof("[CONFIG] A file has been removed, corresponding automations deleted! Path: %s", path)

			// Count
			affectedFiles++

		}
	}

	// Load new or changed Files
	for path := range FilesInFolder {
		if _, found := ConfigFileCopy.AutomationsFiles[path]; !found || ConfigFileCopy.AutomationsFiles[path] != FilesInFolder[path] {

			// Deleting corresponding automations
			DeleteAutomation(path)

			// Read automations
			FileContent, _ := ioutil.ReadFile(path)
			var automationsNew []Automation_t
			yaml.Unmarshal(FileContent, &automationsNew)
			for idNew := range automationsNew {
				AddAutomation(automationsNew[idNew], path)
			}

			// Add File to configFile.Files
			configFileMutex.Lock()
			configFile.AutomationsFiles[path] = FilesInFolder[path]
			configFileMutex.Unlock()

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
				go func(id int) {
					AutomationCopy, ok := CopyAutomation(id)
					for ok {
						<-AutomationCopy.Delay_Timer.C
						automationsMutex.Lock()
						automation, ok := automations[id]
						if ok {
							triggerAction(automation, true)
							automation.Delay_Active = false
						}
						automationsMutex.Unlock()
						if !ok {
							break
						}
					}
				}(id)
			}

			// Add Reminder Ticker
			if Automation.Reminder > 0 {
				Automation.Reminder_Ticker = time.NewTicker(Automation.Reminder)
				Automation.Reminder_Ticker.Stop()
				go func(id int) {
					AutomationCopy, ok := CopyAutomation(id)
					for ok {
						<-AutomationCopy.Reminder_Ticker.C
						automationsMutex.Lock()
						automation, ok := automations[id]
						if ok {
							triggerAction(automation, true)
						}
						automationsMutex.Unlock()
						if !ok {
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

func CheckTriggered(id int, NoTrigger bool) {

	// Check
	AutomationCopy, ok := CopyAutomation(id)
	if !ok {
		return
	}

	triggered := 0

	for Constraint_k := range AutomationCopy.Constraints {
		if CopyConstraint(&AutomationCopy.Constraints[Constraint_k]).Triggered {
			triggered++
		}
	}

	// Set
	automationsMutex.Lock()
	defer automationsMutex.Unlock()
	Automation, ok := automations[id]
	if !ok {
		return
	}

	if AutomationCopy.Mode == "AND" && triggered == len(AutomationCopy.Constraints) {
		if !NoTrigger {
			setTriggered(Automation, true)
		}
	} else if AutomationCopy.Mode == "OR" && triggered >= 1 {
		if !NoTrigger {
			setTriggered(Automation, true)
		}
	} else {
		setTriggered(Automation, false)
	}

}

func setTriggered(Automation *Automation_t, triggered bool) {

	// Debug
	// log.Debugf("SetTriggered automations: %s, Automation.triggered: %t, triggered: %t", Automation.Name, Automation.Triggered, triggered)

	if !triggered {

		// Retrigger
		if !Automation.Triggered && !Automation.Retrigger {
			return
		}

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
		triggerAction(Automation, false)

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
			triggerAction(Automation, true)
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

func triggerAction(Automation *Automation_t, start bool) {

	// log.Debugf("triggerAction automations: %s, start: %t", Automation.Name, start)

	// Create working copy
	AutomationCopy := *Automation
	automationsMutex.Unlock()
	defer automationsMutex.Lock()

	// Run Actions
	for Action_k := range AutomationCopy.Actions {
		if CopyAction(&AutomationCopy.Actions[Action_k]).Trigger != nil {
			if !configFile.Muted {
				AutomationCopy.Actions[Action_k].Mutex.Lock()
				AutomationCopy.Actions[Action_k].Triggered = start
				go AutomationCopy.Actions[Action_k].Trigger(AutomationCopy, Automation.Actions[Action_k])
				if start {
					AutomationCopy.Actions[Action_k].Triggered_Time = time.Now()
				}
				AutomationCopy.Actions[Action_k].Mutex.Unlock()
			}
		}
	}

}

func SetValue(id int, value interface{}) {

	automationsMutex.Lock()
	Automation, ok := automations[id]
	if ok {
		Automation.Value = value
		Automation.Value_Time = time.Now()
	}
	automationsMutex.Unlock()

}

func RTTstart(id int, t time.Time) {

	automationsMutex.Lock()
	Automation, ok := automations[id]
	if ok {
		Automation.RTTstart = t
	}
	automationsMutex.Unlock()

}

func RTTstop(id int) {

	automationsMutex.Lock()
	Automation, ok := automations[id]
	if ok && !Automation.RTTstart.IsZero() {
		Automation.RTTduration = time.Since(Automation.RTTstart)
		Automation.RTTstart = time.Time{}
	}
	automationsMutex.Unlock()

}

func CopyAutomations() map[int]Automation_t {

	automationsMutex.RLock()
	defer automationsMutex.RUnlock()
	ret := make(map[int]Automation_t)
	for id := range automations {
		ret[id] = *automations[id]
	}
	return ret

}

func CopyAutomation(id int) (Automation_t, bool) {

	automationsMutex.RLock()
	defer automationsMutex.RUnlock()
	a, ok := automations[id]
	if ok {
		return *a, true
	} else {
		return Automation_t{}, false
	}

}

func AddAutomation(Automation Automation_t, path string) {

	// Increase Id Counter
	configFileMutex.RLock()
	IdCounter := configFile.IdCounter
	configFile.IdCounter = configFile.IdCounter + 1
	configFileMutex.RUnlock()

	// Add Automation
	automationsMutex.Lock()
	automations[IdCounter] = &Automation
	automations[IdCounter].Id = IdCounter
	automations[IdCounter].File = path
	automationsMutex.Unlock()

}

func DeleteAutomation(file string) {

	// Delete existing automations by File
	automationsMutex.Lock()
	for id := range automations {
		if automations[id].File == file {
			log.Debugf("[CONFIG] Delete Automation! [%p]", automations[id])
			delete(automations, id)
		}
	}
	automationsMutex.Unlock()

}

func CopyConstraint(Constraint *Constraint_t) Constraint_t {

	Constraint.Mutex.RLock()
	defer Constraint.Mutex.RUnlock()
	return *Constraint

}

func CopyAction(Action *Action_t) Action_t {

	Action.Mutex.RLock()
	defer Action.Mutex.RUnlock()
	return *Action

}

func CopyConfigFile() ConfigFile_t {

	configFileMutex.RLock()
	defer configFileMutex.RUnlock()
	return configFile

}

// HELPER //

// Setup Automation_t Defaults
func (s *Automation_t) UnmarshalYAML(unmarshal func(interface{}) error) error {
	defaults.Set(s)
	type plain Automation_t
	if err := unmarshal((*plain)(s)); err != nil {
		return err
	}

	return nil
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
