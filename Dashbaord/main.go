package Dashbaord

import (
	"MqttCommander/Automation"
	"MqttCommander/Config"
	"MqttCommander/Mqtt"
	"MqttCommander/Rule"
	_ "embed"
	"fmt"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
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
	Search                     string
	Automations                []Automation.Automation_t
	AutomationsLen             int
	Files                      []tmplData_File_t
	MemMib                     float32
	NumCPU                     int
	NumGoroutine               int
	BodyOnly                   string
}

type tmplData_File_t struct {
	Path             string
	PathShort        string
	PathShortEscaped string
	Match            bool
	Len              uint
}

func Begin() {

	//	Setup Fiber
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Or extend your config for customization
	/*app.Use(cache.New(cache.Config{
		Expiration: 500 * time.Millisecond,
		//CacheControl: true,
	}))*/

	// Use Compression
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

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
		"automationInPause": func(a Automation.Automation_t) bool {
			return time.Since(a.Triggered_Time) < a.Pause
		},
		"automationInDelay": func(a Automation.Automation_t) bool {
			return time.Since(a.Triggered_Time) < a.Delay
		},
		"getByAutomationId": func(tag string, id uint) []Rule.Rule_t {
			rules := Rule.GetByAutomationTagId(tag, id)
			sort.Slice(rules, func(i, j int) bool { return rules[i].Id < rules[j].Id })
			return rules
		},
		"getTypedValue": func(v interface{}) interface{} {
			if v == nil {
				return nil
			}
			switch v.(type) {
			case bool:
				return fmt.Sprintf("<i>%t</i>", v)
			case float64:
				return fmt.Sprintf("%f", v)
			case string:
				if v.(string) == "" {
					return ""
				} else {
					return "\"" + v.(string) + "\""
				}
			default:
				return "[" + v.(string) + "]"
			}
		},
	}).Parse(index_html)
	//}).ParseFiles("Dashbaord/index.html")
	if err != nil {
		log.Errorf("[DASHBAORD] Template Parsing Error: %s", err)
	}

	// Provide Bootstrap
	app.Get("/bootstrap.min.css", func(c *fiber.Ctx) error {
		c.Set("Content-Type", "text/css; charset=utf-8")
		c.Set("Cache-Control", "public, max-age=999")
		c.SendString(bootstrap_min_css)
		return nil
	})

	// Provide Search Results
	app.Get("/:search?", func(c *fiber.Ctx) error {

		// Query
		var tmplData tmplData_t
		tmplData.ConfigFile = Config.Get()
		tmplData.LocalTime = time.Now().In(tmplData.ConfigFile.Timezone_parsed)
		tmplData.MqttClientIsConnectionOpen = Mqtt.Client.IsConnectionOpen()
		tmplData.Search, _ = url.QueryUnescape(c.Params("search"))
		var MemStats runtime.MemStats
		runtime.ReadMemStats(&MemStats)
		tmplData.MemMib = float32(MemStats.Alloc / 1024 / 1024)
		tmplData.NumCPU = runtime.NumCPU()
		tmplData.NumGoroutine = runtime.NumGoroutine()
		tmplData.BodyOnly = string(c.Request().Header.Peek("body-only"))

		// Get Automations
		Automations := Automation.GetAll()
		tmplData.AutomationsLen = len(Automations)

		// Get Files
		Files := make(map[string]uint)
		for _, v := range Automations {
			_, ok := Files[v.Path]
			if ok {
				Files[v.Path]++
			} else {
				Files[v.Path] = 1
			}
		}

		// Add All Files
		var tmplData_File tmplData_File_t
		tmplData_File.PathShort = "All files!"
		tmplData_File.PathShortEscaped = ""
		tmplData_File.Match = "" == tmplData.Search
		tmplData_File.Len = uint(len(Automations))
		tmplData.Files = append(tmplData.Files, tmplData_File)

		// Sort Files
		for k, v := range Files {
			var tmplData_File tmplData_File_t
			tmplData_File.Path = k
			tmplData_File.PathShort = strings.Replace(k, tmplData.ConfigFile.AutomationsPath+"/", "", 1)
			tmplData_File.PathShortEscaped = url.QueryEscape(tmplData_File.PathShort)
			tmplData_File.Match = tmplData_File.PathShort == tmplData.Search
			tmplData_File.Len = v
			tmplData.Files = append(tmplData.Files, tmplData_File)
		}
		sort.Slice(tmplData.Files, func(i, j int) bool { return tmplData.Files[i].Path < tmplData.Files[j].Path })

		// Filter Automations
		for _, v := range Automations {
			if tmplData.Search == "" || v.Path == tmplData.ConfigFile.AutomationsPath+"/"+tmplData.Search {
				tmplData.Automations = append(tmplData.Automations, v)
			}
		}

		// Sort Automations
		sort.Slice(tmplData.Automations, func(i, j int) bool { return tmplData.Automations[i].Id < tmplData.Automations[j].Id })

		// Send Template
		c.Set("Content-Type", "text/html; charset=utf-8")
		err = Template.Execute(c, tmplData)
		if err != nil {
			log.Errorf("[DASHBAORD] Template Execute Error: %s", err)
		}
		return nil

	})

	// Start Fiber Server
	go func() {
		app.Listen(":9090")
	}()

	// Info
	log.Info("[DASHBAORD] Dashbaord started! Visit http://localhost:9090 🔥")

}
