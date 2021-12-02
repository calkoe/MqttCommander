package Dashbaord

import (
	"MqttCommander/Config"
	"MqttCommander/Mqtt"
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

var Template *template.Template

type tmplData_t struct {
	ConfigFile                 Config.ConfigFile_t
	LocalTime                  time.Time
	MqttClientIsConnectionOpen bool
	ConfigPath                 string
	Search                     string
	Automations                map[int]*Config.Automation_t
	AutomationsFiles           []struct {
		ShortName            string
		Search               bool
		QueryEscapeShortName string
	}
	AutomationsLen int
}

func Init() {

	// Parse Template
	Template, err := template.New("index.html").Funcs(template.FuncMap{
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
				if a.Constraints[Constraint_k].Mqtt != "" {
					return true
				}
			}
			return false
		},
		"automationHaveConstraintCron": func(a Config.Automation_t) bool {
			for Constraint_k, _ := range a.Constraints {
				if a.Constraints[Constraint_k].Cron != "" {
					return true
				}
			}
			return false
		},
		"automationHaveActionMqtt": func(a Config.Automation_t) bool {
			for Action_k, _ := range a.Actions {
				if a.Actions[Action_k].Mqtt != "" {
					return true
				}
			}
			return false
		},
		"automationHaveActionHttp": func(a Config.Automation_t) bool {
			for Action_k, _ := range a.Actions {
				if a.Actions[Action_k].Http != "" {
					return true
				}
			}
			return false
		},
	}).Parse(index_html) //}).ParseFiles("Dashbaord/index.html")
	if err != nil {
		log.Errorf("[DASHBAORD] Template Parsing Error: %s", err)
		return
	}

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
		tmplData.ConfigFile = Config.CopyConfigFile()
		tmplData.LocalTime = time.Now().In(tmplData.ConfigFile.Timezone_parsed)
		tmplData.MqttClientIsConnectionOpen = Mqtt.Client.IsConnectionOpen()
		tmplData.Search, _ = url.QueryUnescape(c.Params("search"))

		// Worker Pool
		worker := make(chan bool, 64)
		workerNum := 0

		// Sort Automations
		AutomationsCopy := Config.CopyAutomations()
		tmplData.AutomationsLen = len(AutomationsCopy)
		ids := make([]int, len(AutomationsCopy))
		i := 0
		for id := range AutomationsCopy {
			ids[i] = id
			i++
		}
		sort.Ints(ids)

		// Copy Automations
		tmplData.Automations = make(map[int]*Config.Automation_t)
		for k, id := range ids {

			if tmplData.Search == "" || strings.Replace(AutomationsCopy[id].File, tmplData.ConfigFile.AutomationsPath+"/", "", 1) == tmplData.Search {

				// Copy Automations by Value
				copy := AutomationsCopy[id]
				tmplData.Automations[k] = &copy

				// Start Data Collection Workers
				go func(worker chan bool, id int, tmplDataAutomation *Config.Automation_t) {

					var Constraints []Config.Constraint_t
					for Constraint_k := range AutomationsCopy[id].Constraints {
						Constraints = append(Constraints, Config.CopyConstraint(&AutomationsCopy[id].Constraints[Constraint_k]))
					}
					tmplDataAutomation.Constraints = Constraints

					var Actions []Config.Action_t
					for Action_k := range AutomationsCopy[id].Actions {
						Actions = append(Actions, Config.CopyAction(&AutomationsCopy[id].Actions[Action_k]))
					}
					tmplDataAutomation.Actions = Actions

					worker <- true

				}(worker, id, tmplData.Automations[k])
				workerNum++

			}

		}

		// Prepare tmplData.AutomationsFiles
		for AutomationsFile := range tmplData.ConfigFile.AutomationsFiles {
			ShortName := strings.Replace(AutomationsFile, tmplData.ConfigFile.AutomationsPath+"/", "", 1)
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

		// Wait for worker Pool
		for i := 0; i < workerNum; i++ {
			<-worker
		}

		// Send Template
		c.Set("Content-Type", "text/html; charset=utf-8")
		err := Template.Execute(c, tmplData)
		if err != nil {
			log.Errorf("[DASHBAORD] Template Execute Error: %s", err)
		}
		return nil

	})

	// Start Server
	go func() {
		app.Listen(":9090")
	}()

	log.Info("[DASHBAORD] Dashbaord started! Visit http://localhost:9090 🔥")

}
