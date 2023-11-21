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
	Backup          bool   `json:"backup"`
	Restore         bool   `json:"restore"`
	Maintenance     bool   `json:"maintenance"`
	Tls             bool   `json:"tls"`
	SettingsKey     string `json:"settingsKey"`
}

//go:embed template/api.txt
var apiGoTemplate embed.FS

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
	} else {
		renderer.NamePluralLower = strings.ToLower(renderer.Name) + "s"
	}
	renderer.NameShort = strings.TrimLeft(strings.ToLower(renderer.Name), "vshn")

	funcMap := template.FuncMap{
		"CamelCase": strcase.ToCamel,
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

}
