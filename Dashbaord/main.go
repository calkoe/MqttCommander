package Dashbaord

import (
	"MqttCommander/Config"
	_ "embed"
	"net/url"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

//go:embed index.html
var index_html string

//go:embed bootstrap.min.css
var bootstrap_min_css string

type tmplData_t struct {
	LocalTime        time.Time
	IdCounter        uint64
	ConfigFile       Config.ConfigFile_t
	Automations      map[uint64]*Config.Automation_t
	ConfigPath       string
	Search           string
	AutomationsFiles []struct {
		ShortName            string
		Search               bool
		QueryEscapeShortName string
	}
}

func Init() {

	//	Dashbaord
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Get("/bootstrap.min.css", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/css; charset=utf-8")
		c.SendString(bootstrap_min_css)
		return nil
	})

	app.Get("/:search?", func(c *fiber.Ctx) error {

		// Query
		var tmplData tmplData_t
		tmplData.LocalTime = time.Now().In(Config.GetConfigFile().Timezone_parsed)
		tmplData.IdCounter = Config.IdCounter
		tmplData.ConfigFile = Config.GetConfigFile()
		tmplData.Automations = make(map[uint64]*Config.Automation_t)
		tmplData.ConfigPath = Config.ConfigPath
		tmplData.Search, _ = url.QueryUnescape(c.Params("search"))

		// Prepare tmplData.AutomationsFiles
		for AutomationsFile := range Config.GetConfigFile().AutomationsFiles {
			ShortName := strings.Replace(AutomationsFile, Config.GetConfigFile().AutomationsPath+"/", "", 1)
			tmplData.AutomationsFiles = append(tmplData.AutomationsFiles,
				struct {
					ShortName            string
					Search               bool
					QueryEscapeShortName string
				}{
					ShortName:            ShortName,
					Search:               ShortName == tmplData.Search,
					QueryEscapeShortName: url.QueryEscape(ShortName),
				})
		}

		// Sort tmplData.AutomationsFiles
		sort.SliceStable(tmplData.AutomationsFiles, func(i, j int) bool {
			return tmplData.AutomationsFiles[i].ShortName > tmplData.AutomationsFiles[j].ShortName
		})
		tmplData.AutomationsFiles = append(tmplData.AutomationsFiles,
			struct {
				ShortName            string
				Search               bool
				QueryEscapeShortName string
			}{
				ShortName:            "All files!",
				Search:               tmplData.Search == "",
				QueryEscapeShortName: url.QueryEscape(""),
			})

		// Prepare Automations
		for id, Automation := range Config.GetAutomations() {
			if tmplData.Search == "" || strings.Replace(Automation.File, Config.GetConfigFile().AutomationsPath+"/", "", 1) == tmplData.Search {
				tmplData.Automations[id] = &Automation
			}
		}

		// Send Template
		c.Set("Content-Type", "text/html; charset=utf-8")
		tmpl, _ := template.New("index.html").Funcs(template.FuncMap{
			"range2s": func(t time.Time) bool {
				return time.Since(t) < 2*time.Second
			},
			"diff": func(t time.Time) string {
				if t.IsZero() {
					return "-"
				} else {
					return t.Sub(time.Now()).Round(time.Second).String()
				}
			},
			"automationInPause": func(a Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Pause
			},
			"automationInDelay": func(a Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Delay
			},
			"automationHaveConstraintMqtt": func(a Config.Automation_t) bool {
				for Constraint_k, _ := range a.Constraints {
					a.Constraints[Constraint_k].Mutex.Lock()
					defer a.Constraints[Constraint_k].Mutex.Unlock()
					if a.Constraints[Constraint_k].Mqtt != "" {
						return true
					}
				}
				return false
			},
			"automationHaveConstraintCron": func(a Config.Automation_t) bool {
				for Constraint_k, _ := range a.Constraints {
					a.Constraints[Constraint_k].Mutex.Lock()
					defer a.Constraints[Constraint_k].Mutex.Unlock()
					if a.Constraints[Constraint_k].Cron != "" {
						return true
					}
				}
				return false
			},
			"automationHaveActionMqtt": func(a Config.Automation_t) bool {
				for Action_k, _ := range a.Actions {
					a.Actions[Action_k].Mutex.Lock()
					defer a.Actions[Action_k].Mutex.Unlock()
					if a.Actions[Action_k].Mqtt != "" {
						return true
					}
				}
				return false
			},
			"automationHaveActionHttp": func(a Config.Automation_t) bool {
				for Action_k, _ := range a.Actions {
					a.Actions[Action_k].Mutex.Lock()
					defer a.Actions[Action_k].Mutex.Unlock()
					if a.Actions[Action_k].Http != "" {
						return true
					}
				}
				return false
			},
		}).Parse(index_html)
		//}).ParseFiles("Dashbaord/index.html")
		err := tmpl.Execute(c, tmplData)
		if err != nil {
			log.Errorf("[DASHBAORD] Template Error: %s", err)
		}

		return nil

	})

	// Start Server
	go func() {
		app.Listen(":9090")
	}()

	log.Info("[DASHBAORD] Dashbaord started! Visit http://localhost:9090 🔥")

}
