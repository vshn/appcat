package slareport

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"
)

type Renderer interface {
	RenderAsciidoc() (string, error)
	PrepareJSONPayload() ([]byte, error)
}

type DocGenPDF struct {
	Asciidoc             string `json:"asciidoc,omitempty"`
	VshnDocgenId         string `json:"vshn_docgen_id,omitempty"`
	VshnTextRoleOfVshnAg string `json:"vshn_text_role_of_vshn_ag,omitempty"`
}

//go:embed template/sla-report.txt
var slaReportGoTemplate embed.FS
var appcatSLAReport = "appcat-sla-report"

type ServiceInstance struct {
	Namespace  string
	Instance   string
	TargetSLA  float64
	OutcomeSLA float64
	Cluster    string
	Service    string
	Color      string
}

type SLARenderer struct {
	Customer      string
	ExceptionLink string
	Month         time.Month
	Year          int
	SI            []ServiceInstance
}

// RenderAsciidoc renders the sla go template into an asciidoc template
func (s *SLARenderer) RenderAsciidoc() (string, error) {
	t, err := template.ParseFS(slaReportGoTemplate, "template/sla-report.txt")
	if err != nil {
		return "", fmt.Errorf("cannot parse sla-report go template: %v", err)
	}

	buf := new(bytes.Buffer)

	err = t.Execute(buf, s)
	if err != nil {
		return "", fmt.Errorf("cannot render sla-report go template: %v", err)
	}
	return buf.String(), nil
}

// PrepareJSONPayload creates a json payload that is ready to be used for docgen api
// endpoint https://docgen.vshn.net/api/pdf
func (s *SLARenderer) PrepareJSONPayload() ([]byte, error) {
	asciidocTemplate, err := s.RenderAsciidoc()
	if err != nil {
		return nil, err
	}

	d := DocGenPDF{
		Asciidoc:     asciidocTemplate,
		VshnDocgenId: appcatSLAReport,
	}

	payload, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal sla-report json payload: %v", err)
	}

	return payload, nil
}

// GeneratePDF sends a request to docgen an returns the rendered PDF.
func (s *SLARenderer) GeneratePDF() (io.ReadCloser, error) {

	reqJson, err := s.PrepareJSONPayload()
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(reqJson)

	req, err := http.NewRequest("POST", "https://docgen.vshn.net/api/pdf", reader)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res.Body, nil
}
