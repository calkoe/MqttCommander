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
	ConfigFile       Config.ConfigFile_t
	LocalTime        time.Time
	Automations      map[int]*Config.Automation_t
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
		tmplData.ConfigFile = Config.CopyConfigFile()
		tmplData.LocalTime = time.Now().In(tmplData.ConfigFile.Timezone_parsed)
		tmplData.Automations = make(map[int]*Config.Automation_t)
		tmplData.Search, _ = url.QueryUnescape(c.Params("search"))

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

		// Prepare Automations
		AutomationCopy := Config.CopyAutomations()
		keys := make([]int, len(AutomationCopy))
		i := 0
		for k := range AutomationCopy {
			keys[i] = k
			i++
		}
		sort.Ints(keys)

		for _, k := range keys {
			if tmplData.Search == "" || strings.Replace(AutomationCopy[k].File, tmplData.ConfigFile.AutomationsPath+"/", "", 1) == tmplData.Search {

				d := AutomationCopy[k]
				tmplData.Automations[k] = &d
				var Constraints []Config.Constraint_t
				for Constraint_k := range AutomationCopy[k].Constraints {
					Constraints = append(Constraints, Config.CopyConstraint(&AutomationCopy[k].Constraints[Constraint_k]))
				}
				tmplData.Automations[k].Constraints = Constraints

				var Actions []Config.Action_t
				for Action_k := range AutomationCopy[k].Actions {
					Actions = append(Actions, Config.CopyAction(&AutomationCopy[k].Actions[Action_k]))
				}
				tmplData.Automations[k].Actions = Actions

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
