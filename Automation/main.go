package Automation

import (
	"MqttCommander/Rule"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creasty/defaults"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Automation_t struct {
	Id               uint
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
	Constraints      []struct {
		Mqtt string `yaml:"Mqtt"`
		Cron string `yaml:"Cron"`
	} `yaml:"Constraints"`
	Actions []struct {
		Mqtt string `yaml:"Mqtt"`
		Http string `yaml:"Http"`
	} `yaml:"Actions"`
	Triggered      bool `yaml:"Triggered"`
	Path           string
	Triggered_Time time.Time
	Value          interface{}
	Value_Time     time.Time
	RTTstart       time.Time
	RTTduration    time.Duration
	Initialized    bool
}

var idCounter uint
var automations map[uint]*Automation_t
var automationsFiles map[string]string

// var mutex deadlock.RWMutex
var mutex sync.RWMutex

// PUBLIC //

// Begin
func Begin() {

	mutex.Lock()
	defer mutex.Unlock()

	automations = make(map[uint]*Automation_t)
	automationsFiles = make(map[string]string)

}

// Read Automations from File
func Read(AutomationsPath string) (affectedFiles int) {

	mutex.Lock()
	defer mutex.Unlock()

	// List automations Files
	FilesInFolder := make(map[string]string)
	err := filepath.Walk(AutomationsPath, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".yml" {
			file, err := os.Open(path)
			if err != nil {
				log.Errorf("[AUTOMATION] Problem opening File %s", err)
			}
			hash := sha1.New()
			io.Copy(hash, file)
			FilesInFolder[path] = hex.EncodeToString(hash.Sum(nil))
			file.Close()
		}
		return nil
	})
	if err != nil {
		log.Errorf("[AUTOMATION] walk error [%v]\n", err)
	}

	// Check if files were removed
	for path := range automationsFiles {
		if _, found := FilesInFolder[path]; !found {

			// Deleting corresponding automations
			removeByPath(path)

			// Delete File from automationsFiles
			delete(automationsFiles, path)

			// Log
			log.Infof("[AUTOMATION] A file has been removed, corresponding automations deleted! Path: %s", path)

			// Count
			affectedFiles++

		}
	}

	// Load new or changed Files
	for path := range FilesInFolder {
		if _, found := automationsFiles[path]; !found || automationsFiles[path] != FilesInFolder[path] {

			// Deleting corresponding automations
			removeByPath(path)

			// Read automations
			FileContent, _ := ioutil.ReadFile(path)
			var automationsNew []Automation_t
			yaml.Unmarshal(FileContent, &automationsNew)
			for idNew := range automationsNew {
				add(automationsNew[idNew], path)
			}

			// Add to automationsFiles
			automationsFiles[path] = FilesInFolder[path]

			// Log
			log.Infof("[AUTOMATION] read %d automations from file: %s [Hash: %s]", len(automationsNew), path, FilesInFolder[path])

			// Count
			affectedFiles++

		}
	}

	return affectedFiles

}

// Deploy Automations
func Deploy() {

	mutex.Lock()
	defer mutex.Unlock()

	// Setup automations
	for id := range automations {

		Automation := automations[id]

		if !Automation.Initialized {
			Automation.Initialized = true

			// Log
			log.WithFields(log.Fields{
				"Id":          id,
				"Name":        Automation.Name,
				"Path":        Automation.Path,
				"Constraints": len(Automation.Constraints),
				"Actions":     len(Automation.Actions),
			}).Debugf("[CONFIG] Deploy Automation [%04d]", id)

			// Add Delay Timer
			if Automation.Delay > 0 {
				Automation.Delay_Timer = time.NewTimer(Automation.Delay)
				Automation.Delay_Timer.Stop()
				go delayTimerTask(id)
			}

			// Add Reminder Ticker
			if Automation.Reminder > 0 {
				Automation.Reminder_Ticker = time.NewTicker(Automation.Reminder)
				Automation.Reminder_Ticker.Stop()
				go reminderTickerTask(id)
			}

			// If inital triggered
			if Automation.Triggered {
				Automation.Triggered = false
				Automation.setTrigger(true)
			}

		}
	}
}

// Check if a Automation should be triggered
func CheckTriggered(id uint, NoTrigger bool) {

	mutex.Lock()
	defer mutex.Unlock()

	Automation, ok := automations[id]
	if !ok {
		return
	}

	total1, triggered1 := Rule.CountTriggeredByAutomationTagId("constraint/cron", id)
	total2, triggered2 := Rule.CountTriggeredByAutomationTagId("constraint/mqtt", id)

	// Set
	if Automation.Mode == "AND" && triggered1+triggered2 == total1+total2 {
		if !NoTrigger {
			Automation.setTrigger(true)
		}
	} else if Automation.Mode == "OR" && triggered1+triggered2 >= 1 {
		if !NoTrigger {
			Automation.setTrigger(true)
		}
	} else {
		Automation.setTrigger(false)
	}

}

// Get Automation
func Get(id uint) (Automation_t, bool) {

	mutex.Lock()
	defer mutex.Unlock()

	automation, ok := automations[id]
	if ok {
		return *automation, true
	} else {
		return Automation_t{}, false
	}

}

// Get all Automations
func GetAll() []Automation_t {

	mutex.Lock()
	defer mutex.Unlock()

	ret := []Automation_t{}
	for id := range automations {
		ret = append(ret, *automations[id])
	}
	return ret

}

// Set Automation Value
func SetValue(id uint, value interface{}) {

	mutex.Lock()
	defer mutex.Unlock()

	Automation, ok := automations[id]
	if ok {
		Automation.Value = value
		Automation.Value_Time = time.Now()
	}

}

// Start RTT Measurement
func RTTstart(id uint, t time.Time) {

	mutex.Lock()
	defer mutex.Unlock()

	Automation, ok := automations[id]
	if ok {
		Automation.RTTstart = t
	}

}

// Set Automation Delay Active
func SetDelayActive(id uint, active bool) {

	mutex.Lock()
	defer mutex.Unlock()

	Automation, ok := automations[id]
	if ok {
		Automation.Delay_Active = active
	}

}

// Set Automation Triggered (Public Version)
func SetTrigger(id uint, setTrigger bool) {

	mutex.Lock()
	defer mutex.Unlock()

	Automation, ok := automations[id]
	if ok {
		Automation.setTrigger(setTrigger)
	}

}

// PRIVATE //

// Set Automation Triggered
func (Automation *Automation_t) setTrigger(trigger bool) {

	if !trigger {

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
		StartTriggerFunc(*Automation, false)

	}

	if trigger {

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
			StartTriggerFunc(*Automation, true)
		}

		// Setting Reminder
		if Automation.Reminder > 0 {
			Automation.Reminder_Ticker.Reset(Automation.Reminder)
			Automation.Reminder_Active = true
		}

	}

	//	Log
	if Automation.Triggered != trigger {
		log.WithFields(log.Fields{
			"Id":        Automation.Id,
			"Name":      Automation.Name,
			"Path":      Automation.Path,
			"Value":     Automation.Value,
			"Triggered": trigger,
		}).Debugf("[CONFIG] Automation changed [%04d]", Automation.Id)
	}

	// Set Automation Trigger
	Automation.Triggered = trigger

	// RTTstop
	if !Automation.RTTstart.IsZero() {
		Automation.RTTduration = time.Since(Automation.RTTstart)
		Automation.RTTstart = time.Time{}
	}

}

func StartTriggerFunc(automation Automation_t, trigger bool) {

	for _, rule := range Rule.GetByAutomationId(automation.Id) {

		// Trigger Rule
		if rule.TriggerFunc != nil {
			Rule.SetTrigger(rule.Id, trigger)
			rule.TriggerFunc(rule.Id, automation)
		}

	}

}

// Add Automation
func add(NewAutomation Automation_t, path string) {

	// Add Automations Rules
	for _, v := range NewAutomation.Constraints {
		if v.Cron != "" {
			Rule.Add("constraint/cron", idCounter, v.Cron)
		}
		if v.Mqtt != "" {
			Rule.Add("constraint/mqtt", idCounter, v.Mqtt)
		}
	}
	for _, v := range NewAutomation.Actions {
		if v.Http != "" {
			Rule.Add("action/http", idCounter, v.Http)
		}
		if v.Mqtt != "" {
			Rule.Add("action/mqtt", idCounter, v.Mqtt)
		}
	}

	// Add
	NewAutomation.Id = idCounter
	NewAutomation.Path = path
	automations[idCounter] = &NewAutomation
	idCounter++

	// Debug
	// log.Debugf("[AUTOMATION] Automation added! [%v]", NewAutomation.Id)

}

// Delete existing automations by File
func removeByPath(path string) {

	for id, automation := range automations {
		if automation.Path == path {

			// Remove Automations Rules
			Rule.RemoveByAutomationId(id)

			// Delete
			delete(automations, id)

			// Debug
			// log.Debugf("[AUTOMATION] Automation removed! [%v]", k)

		}
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
