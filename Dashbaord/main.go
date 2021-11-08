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
	Config           Config.Config_t
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
		tmplData.Config = Config.Config
		tmplData.Search, _ = url.QueryUnescape(c.Params("search"))

		// Prepare tmplData.AutomationsFiles
		for AutomationsFile := range Config.Config.AutomationsFiles {
			ShortName := strings.Replace(AutomationsFile, Config.Config.AutomationsPath+"/", "", 1)
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

		// Filter Automations by File
		if tmplData.Search != "" {
			tmplData.Config.Automations = nil
			for _, Automation := range Config.Config.Automations {
				if strings.Replace(Automation.File, Config.Config.AutomationsPath+"/", "", 1) == tmplData.Search {
					tmplData.Config.Automations = append(tmplData.Config.Automations, Automation)
				}
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
			"automationInPause": func(a *Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Pause
			},
			"automationInDelay": func(a *Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Delay
			},
			"automationHaveConstraintMqtt": func(a *Config.Automation_t) bool {
				for _, Constraint := range a.Constraints {
					if Constraint.Mqtt != "" {
						return true
					}
				}
				return false
			},
			"automationHaveConstraintCron": func(a *Config.Automation_t) bool {
				for _, Constraint := range a.Constraints {
					if Constraint.Cron != "" {
						return true
					}
				}
				return false
			},
			"automationHaveActionMqtt": func(a *Config.Automation_t) bool {
				for _, Action := range a.Actions {
					if Action.Mqtt != "" {
						return true
					}
				}
				return false
			},
			"automationHaveActionHttp": func(a *Config.Automation_t) bool {
				for _, Action := range a.Actions {
					if Action.Http != "" {
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
