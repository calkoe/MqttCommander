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

	app.Get("/:file?", func(c *fiber.Ctx) error {

		// Query
		var tmplData tmplData_t
		tmplData.Config = Config.Config
		tmplData.Search, _ = url.QueryUnescape(c.Params("file"))

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
			"automationInPause": func(a *Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Pause
			},
			"automationInDelay": func(a *Config.Automation_t) bool {
				return time.Since(a.Triggered_Time) < a.Delay
			},
		}).Parse(index_html)
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
