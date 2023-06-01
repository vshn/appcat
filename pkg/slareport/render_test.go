package slareport

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSLARenderer_RenderAsciidoc(t *testing.T) {
	tests := map[string]struct {
		renderer         SLARenderer
		expectedAsciidoc string
		err              error
	}{
		"WhenSLARender_ThenOutput": {
			renderer: SLARenderer{
				Customer:      "TestCustomer",
				ExceptionLink: "https://vshn.ch",
				Month:         2,
				SI: []ServiceInstance{
					{
						Namespace:  "dev-namespace",
						Instance:   "postgres-dev",
						TargetSLA:  99.9,
						OutcomeSLA: 99.1,
						Color:      "red",
					},
				},
			},
			expectedAsciidoc: "= SLA Report\n\nimage::vshn.png[VSHN Logo,100,54,id=vshn_logo]\n\n---\n\n[big]#Customer: *TestCustomer* +\nMonth: *February* +\n" +
				"Year: *0*#\n\n---\n\n[cols=\"Namespace, Instance, SLA Target, SLA Outcome\"]\n|===\n|Cluster| " +
				"Service | Namespace| Instance| SLA Target| SLA Outcome\n\n|||dev-namespace|postgres-dev|99.9%|*99." +
				"10%*\n\n|===\n\nNOTE: [small]#The list of exceptions which are excluded from outcome can be viewed  " +
				"https://vshn.ch[at].#\n\n",
			err: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// WHEN
			asciidoc, err := tc.renderer.RenderAsciidoc()

			// THEN
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedAsciidoc, asciidoc)
		})
	}
}

func TestSLARenderer_PrepareJSONPayload(t *testing.T) {
	tests := map[string]struct {
		renderer        SLARenderer
		expectedPayload string
		err             error
	}{
		"WhenSLARender_ThenOutputPayload": {
			renderer: SLARenderer{
				Customer:      "TestCustomer",
				ExceptionLink: "https://vshn.ch",
				Month:         2,
				SI: []ServiceInstance{
					{
						Namespace:  "dev-namespace",
						Instance:   "postgres-dev",
						TargetSLA:  99.9,
						OutcomeSLA: 99.1,
						Color:      "red",
					},
				},
			},
			expectedPayload: `{"asciidoc":"= SLA Report\n\nimage::vshn.png[VSHN Logo,100,54,id=vshn_logo]\n` +
				`\n---\n\n[big]#Customer: *TestCustomer* +\nMonth: ` +
				`*February* +\nYear: *0*#\n\n---\n\n[cols=\"Namespace, Instance, SLA Target, SLA Outcome\"]\n` +
				`|===\n|Cluster| Service | Namespace| Instance| SLA Target| SLA Outcome\n\n|||dev-namespace|postgres-dev` +
				`|99.9%|*99.10%*\n\n|===\n\nNOTE: [small]#The list of exceptions which are excluded from outcome ` +
				`can be viewed  https://vshn.ch[at].#\n\n","vshn_docgen_id":"appcat-sla-report"}`,
			err: nil,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			// WHEN
			payload, err := tc.renderer.PrepareJSONPayload()

			// THEN
			if tc.err != nil {
				assert.Equal(t, tc.err, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedPayload, string(payload))
		})
	}
}
