package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
)

type Service struct {
	Name            string `json:"name"`
	NamePluralLower string
	NameShort       string
	Security        bool   `json:"security"`
	Backup          bool   `json:"backup"`
	Restore         bool   `json:"restore"`
	Maintenance     bool   `json:"maintenance"`
	Tls             bool   `json:"tls"`
	SettingsKey     string `json:"settingsKey"`
	Monitoring      bool   `json:"monitoring"`
	HA              bool   `json:"ha"`
	WorkloadName    string `json:"workloadName"`
}

//go:embed template/api.txt
var apiGoTemplate embed.FS

//go:embed template/webhook.txt
var webhookGoTemplate embed.FS

// This tool will generate the API definition for a new AppCat service.
// It expects a json file as an argument that contains all the necessary
// information to bootstrap the API definition for the new service.
// Check the docs for detailed information.
func main() {
	args := os.Args[1:]

	f, err := os.Open(args[0])
	byteValue, _ := io.ReadAll(f)

	if err != nil {
		fmt.Println(fmt.Errorf("cannot open config file: %v", err))
	}

	var renderer Service

	err = json.Unmarshal(byteValue, &renderer)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot parse json in config file: %v", err))
	}

	if renderer.Name[len(renderer.Name)-1:] == "s" {
		renderer.NamePluralLower = strings.ToLower(renderer.Name)
	} else if renderer.Name[len(renderer.Name)-4:] == "gejo" {
		renderer.NamePluralLower = strings.ToLower(renderer.Name) + "es"
	} else {
		renderer.NamePluralLower = strings.ToLower(renderer.Name) + "s"
	}
	renderer.NameShort = strings.TrimLeft(strings.ToLower(renderer.Name), "vshn")
	renderer.NameShort = strings.TrimLeft(renderer.Name, "VSHN")

	funcMap := template.FuncMap{
		"CamelCase": strcase.ToCamel,
		"ToLower":   strings.ToLower,
	}
	t, err := template.New("api.txt").Funcs(funcMap).ParseFS(apiGoTemplate, "template/api.txt")

	if err != nil {
		fmt.Println(fmt.Errorf("cannot parse api go template: %v", err))
	}
	buf := new(bytes.Buffer)

	err = t.ExecuteTemplate(buf, "api.txt", renderer)

	if err != nil {
		fmt.Println(fmt.Errorf("cannot render api go template: %v", err))
	}

	apiFile := "apis/vshn/v1/dbaas_vshn_" + renderer.NameShort + ".go"
	err = os.WriteFile(apiFile, buf.Bytes(), 0644)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot write file '%s': %v", apiFile, err))
	}

	t, err = template.New("template/webhook.txt").Funcs(funcMap).ParseFS(webhookGoTemplate, "template/webhook.txt")

	if err != nil {
		fmt.Println(fmt.Errorf("cannot parse webhook go template: %v", err))
	}
	buf = new(bytes.Buffer)

	fmt.Println(renderer)

	err = t.ExecuteTemplate(buf, "template/webhook.txt", renderer)

	if err != nil {
		fmt.Println(fmt.Errorf("cannot render webhook go template: %v", err))
	}

	webhookFile := "pkg/webhooks/" + renderer.NameShort + ".go"
	err = os.WriteFile(webhookFile, buf.Bytes(), 0644)
	if err != nil {
		fmt.Println(fmt.Errorf("cannot write file '%s': %v", webhookFile, err))
	}

}
