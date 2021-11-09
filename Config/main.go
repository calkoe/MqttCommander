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
	"time"

	"github.com/creasty/defaults"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorhill/cronexpr"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config_t struct {
	System           System_t       `yaml:"System"`
	Automations      []Automation_t `yaml:"Automations"`
	AutomationsPath  string         `yaml:"AutomationsPath"`
	AutomationsFiles map[string]string
	ConfigPath       string
}

type System_t struct {
	Name    string `yaml:"Name"`
	MqttUri string `yaml:"MqttUri"`
}

type Automation_t struct {
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
}

type Action_t struct {
	Mqtt        string `yaml:"Mqtt"`
	Mqtt_Parsed struct {
		Topic    string
		Object   string
		Value    string
		Retained bool
	}
	Http           string `yaml:"Http"`
	Trigger        func()
	Triggered      bool
	Triggered_Time time.Time
	Initialized    bool
}

var Config Config_t

//go:embed config.yml
var config_yml []byte

//go:embed Automations/automation.yml
var automation_yml []byte

func SetupConfig(ConfigPath string) {

	// Set ConfigPath
	if ConfigPath == "" {

		// Automatic
		ex, err := os.Executable()
		if err != nil {
			panic(err)
		}
		Config.ConfigPath = filepath.Dir(ex) + "/Config"
		log.Infof("[CONFIG] ConfigPath has been automatically set to: %s", Config.ConfigPath)

	} else {

		//Manual
		Config.ConfigPath = ConfigPath
		log.Infof("[CONFIG] ConfigPath has been manually set to: %s", Config.ConfigPath)

	}

	// Create AutomationsFiles
	Config.AutomationsFiles = make(map[string]string)

}

func ReadConfig() {

	// Check Config.ConfigPath
	if _, err := os.Stat(Config.ConfigPath); err != nil {
		if os.IsNotExist(err) {
			log.Warn("[CONFIG] " + Config.ConfigPath + " does not exits, creating it!")
			os.Mkdir(Config.ConfigPath, 0744)
			log.Warn("[CONFIG] " + Config.ConfigPath + "/config.yml does not exits, creating it!")
			os.WriteFile(Config.ConfigPath+"/config.yml", config_yml, 0644)
		}
	}

	// Read Main Config File (without Automations!)
	yamlFile, err := ioutil.ReadFile(Config.ConfigPath + "/config.yml")
	if err != nil {
		panic(err)
	}
	yaml.Unmarshal(yamlFile, &Config)
	Config.Automations = []Automation_t{}
	log.Infof("[CONFIG] Successfully read main config file: %s", Config.ConfigPath+"/config.yml")

	// Check Config.ConfigPath/Automations
	Config.AutomationsPath = Config.ConfigPath + "/Automations"
	log.Infof("[CONFIG] Set AutomationsPath to: %s", Config.AutomationsPath)
	if _, err := os.Stat(Config.AutomationsPath); err != nil {
		if os.IsNotExist(err) {
			log.Warnf("[CONFIG] %S does not exits, creating it!", Config.AutomationsPath)
			os.Mkdir(Config.AutomationsPath, 0744)
			log.Warnf("[CONFIG] %s/automation.yml does not exits, creating it!", Config.AutomationsPath)
			os.WriteFile(Config.AutomationsPath+"/automation.yml", automation_yml, 0644)
		}
	}

}

func ReadAutomations() uint {

	var affectedFiles uint

	// List Automation Files
	FilesInFolder := make(map[string]string)
	err := filepath.Walk(Config.AutomationsPath, func(path string, info os.FileInfo, err error) error {
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
	for path := range Config.AutomationsFiles {
		if _, found := FilesInFolder[path]; !found {

			// Log
			log.Infof("[CONFIG] A file has been removed, deleting corresponding automations! Path: %s", path)

			// Deleting corresponding Automations
			deletebyFile(path)

			// Delete File from Config.Files
			delete(Config.AutomationsFiles, path)

			// Count
			affectedFiles++

		}
	}

	// Load new Files
	for path := range FilesInFolder {
		if _, found := Config.AutomationsFiles[path]; !found || Config.AutomationsFiles[path] != FilesInFolder[path] {

			// Deleting corresponding Automations
			deletebyFile(path)

			// Read Automations
			FileContent, _ := ioutil.ReadFile(path)
			var Automations []Automation_t
			yaml.Unmarshal(FileContent, &Automations)
			for Automation_k := range Automations {
				Automations[Automation_k].File = path
			}
			Config.Automations = append(Config.Automations, Automations...)

			// Add File to Config.Files
			Config.AutomationsFiles[path] = FilesInFolder[path]

			// Log
			log.Infof("[CONFIG] read %d automations from file: %s [Hash: %s]", len(Automations), path, FilesInFolder[path])

			// Count
			affectedFiles++

		}

	}

	return affectedFiles

}

func Deploy() {

	// Setup Automations
	for Automation_k := range Config.Automations {
		Automation := &Config.Automations[Automation_k]

		if !Automation.Initialized {
			Automation.Initialized = true

			// Log
			log.WithFields(log.Fields{
				"Name":        Automation.Name,
				"File":        strings.Replace(Automation.File, Config.AutomationsPath+"/", "", 1),
				"Constraints": len(Automation.Constraints),
				"Actions":     len(Automation.Actions),
			}).Debug("[CONFIG] Initialize Automation")

			// Add Delay Timer
			if Automation.Delay > 0 {
				Automation.Delay_Timer = time.NewTimer(Automation.Delay)
				Automation.Delay_Timer.Stop()
				go func() {
					Automation_c := Automation
					for {
						<-Automation_c.Delay_Timer.C
						if !Automation_c.Initialized {
							Automation_c.Delay_Timer.Stop()
							return
						}
						runActions(Automation_c)
						Automation.Delay_Active = false
					}
				}()
			}

			// Add Reminder Ricker
			if Automation.Reminder > 0 {
				Automation.Reminder_Ticker = time.NewTicker(Automation.Reminder)
				Automation.Reminder_Ticker.Stop()
				go func() {
					Automation_c := Automation
					for {
						<-Automation_c.Reminder_Ticker.C
						if !Automation_c.Initialized {
							Automation_c.Reminder_Ticker.Stop()
							return
						}
						runActions(Automation_c)
					}
				}()
			}

			// If inital triggered
			if Automation.Triggered {
				Automation.Triggered = false
				setTriggered(Automation, true)
			}

		}
	}
}

func CheckTriggered(Automation *Automation_t, NoTrigger bool) {

	// Debug
	// log.Debugf("CheckTriggered Automation: %s", Automation.Name)

	triggered := 0
	for Constraint_k := range Automation.Constraints {
		Constraint := &Automation.Constraints[Constraint_k]
		if Constraint.Triggered {
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
		stopActions(Automation)

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
			runActions(Automation)
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
			"Name":      Automation.Name,
			"File":      strings.Replace(Automation.File, Config.AutomationsPath+"/", "", 1),
			"Value":     Automation.Value,
			"Triggered": triggered,
		}).Debug("[CONFIG] Automation changed")
	}

	Automation.Triggered = triggered
}

func runActions(Automation *Automation_t) {
	for Action_k := range Automation.Actions {
		Action := &Automation.Actions[Action_k]
		if Action.Trigger != nil {
			Action.Trigger()
			Action.Triggered = true
			Action.Triggered_Time = time.Now()
		}
	}
}

func stopActions(Automation *Automation_t) {
	for Action_k := range Automation.Actions {
		Action := &Automation.Actions[Action_k]
		Action.Triggered = false
	}
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

func deletebyFile(file string) {

	var Automations []Automation_t

	// Delete existing Automations by File
	for Automation_k := range Config.Automations {
		Automation := &Config.Automations[Automation_k]
		if Automation.File == file {
			Automation.Initialized = false
			for Constraint_k := range Automation.Constraints {
				Constraint := &Automation.Constraints[Constraint_k]
				Constraint.Initialized = false
			}
		} else {
			Automations = append(Automations, Config.Automations[Automation_k])
		}
	}

	Config.Automations = Automations

}

func Find(search string, data string) (match string) {
	exp, err := regexp.Compile(search)
	if err != nil {
		log.Errorf("[MQTT] Error while Parsing Regex: %s", err)
	}
	results := exp.FindStringSubmatch(data)
	if len(results) == 2 {
		match = results[1]
	}
	return
}
