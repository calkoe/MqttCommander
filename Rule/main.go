package Rule

import (
	"fmt"
	"sync"
	"time"

	"github.com/sasha-s/go-deadlock"
	log "github.com/sirupsen/logrus"
)

type Rule_t struct {
	Id             uint
	Tag            string
	AutomationId   uint
	Text           string
	TriggerFunc    func(uint)
	Triggered      bool
	Triggered_Time time.Time
	Value          interface{}
	Value_Time     time.Time
	Initialized    bool
	Mutex          deadlock.RWMutex
	Module         interface{}
	Error          string
}

var idCounter uint
var rules map[uint]*Rule_t

//var mutex deadlock.RWMutex
var mutex sync.RWMutex

// PUBLIC //

// Begin
func Begin() {

	mutex.Lock()
	defer mutex.Unlock()

	rules = make(map[uint]*Rule_t)

}

// Get
func Get(id uint) (Rule_t, bool) {

	mutex.RLock()
	defer mutex.RUnlock()

	rule, ok := rules[id]
	if ok {
		return *rule, true
	} else {
		return Rule_t{}, false
	}

}

func GetAll() []Rule_t {

	mutex.RLock()
	defer mutex.RUnlock()

	ret := []Rule_t{}
	for k, _ := range rules {
		ret = append(ret, *rules[k])
	}
	return ret

}

func GetAllByTag(Tag string) []Rule_t {

	mutex.RLock()
	defer mutex.RUnlock()

	ret := []Rule_t{}
	for k, rule := range rules {
		if rule.Tag == Tag {
			ret = append(ret, *rules[k])
		}
	}
	return ret

}

func GetByAutomationId(AutomationId uint) []Rule_t {

	mutex.RLock()
	defer mutex.RUnlock()

	ret := []Rule_t{}
	for k, rule := range rules {
		if rule.AutomationId == AutomationId {
			ret = append(ret, *rules[k])
		}
	}
	return ret

}

func GetByAutomationTagId(Tag string, AutomationId uint) []Rule_t {

	mutex.RLock()
	defer mutex.RUnlock()

	ret := []Rule_t{}
	for k, rule := range rules {
		if rule.Tag == Tag && rule.AutomationId == AutomationId {
			ret = append(ret, *rules[k])
		}
	}
	return ret

}

func CountTriggeredByAutomationTagId(Tag string, AutomationId uint) (total uint, triggered uint) {

	mutex.RLock()
	defer mutex.RUnlock()

	for _, rule := range rules {
		if rule.Tag == Tag && rule.AutomationId == AutomationId {
			total++
			if rule.Triggered {
				triggered++
			}
		}
	}
	return

}

// Trigger
func SetTriggerFunc(id uint, triggerFunc func(uint)) {

	mutex.Lock()
	defer mutex.Unlock()

	_, ok := rules[id]
	if ok {
		rules[id].TriggerFunc = triggerFunc
	}

}

func SetTrigger(id uint, trigger bool) {

	mutex.Lock()
	defer mutex.Unlock()

	_, ok := rules[id]
	if ok {
		rules[id].Triggered = trigger

		// Set Last Triggered
		if trigger {
			rules[id].Triggered_Time = time.Now()
		}
	}

}

// Value
func SetValue(id uint, value interface{}) {

	mutex.Lock()
	defer mutex.Unlock()

	_, ok := rules[id]
	if ok {
		rules[id].Value_Time = time.Now()
		rules[id].Value = value
	}

}

func SetModule(id uint, module interface{}) {

	mutex.Lock()
	defer mutex.Unlock()

	_, ok := rules[id]
	if ok {
		rules[id].Module = module
		rules[id].Initialized = true
	}

}

// Stats
func SetError(id uint, err string, args ...interface{}) {

	mutex.Lock()
	defer mutex.Unlock()

	_, ok := rules[id]
	if ok {
		rules[id].Error = fmt.Sprintf(err, args...)
		if rules[id].Error != "" {
			log.WithFields(log.Fields{
				"Id":    rules[id].Id,
				"Text":  rules[id].Text,
				"Error": rules[id].Error,
			}).Errorf("[RULE] Error in Rule [%06d]", rules[id].Id)
		}

	}

}

// Add / Remove Rules
func Add(Tag string, AutomationId uint, Text string) uint {

	mutex.Lock()
	defer mutex.Unlock()

	rule_v := Rule_t{}
	rule_v.Id = idCounter
	rule_v.Tag = Tag
	rule_v.AutomationId = AutomationId
	rule_v.Text = Text
	rules[idCounter] = &rule_v
	idCounter++
	return idCounter - 1

}

func Remove(id uint) {

	mutex.Lock()
	defer mutex.Unlock()
	delete(rules, id)

}

func RemoveByAutomationId(AutomationId uint) {

	mutex.Lock()
	defer mutex.Unlock()

	for k, rule := range rules {
		if rule.AutomationId == AutomationId {
			delete(rules, k)
		}
	}
}
