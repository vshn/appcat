package reporting

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
				Cluster:       "TestCluster",
				ExceptionLink: "https://vshn.ch",
				Month:         2,
				SI: []ServiceInstance{
					{
						Namespace:  "dev-namespace",
						Instance:   "postgres-dev",
						TargetSLA:  99.9,
						OutcomeSLA: 99.1,
					},
				},
			},
			expectedAsciidoc: "= SLA Report\n\nimage::vshn.png[VSHN Logo,100,54,id=vshn_logo]\n\n*Version of the document" +
				": May 2023* +\n\n---\n\n[big]#Customer: *TestCustomer* +\nMonth: *February* +\nCluster: *TestCluster*#\n" +
				"\n---\n\n[cols=\"Namespace, Instance, SLA Target, SLA Outcome\"]\n|===\n| Namespace| Instance| SLA Target|" +
				" SLA Outcome\n\n|dev-namespace|postgres-dev|99.9%|*99.1%*\n\n|===\n\nNOTE: [small]#The list of exceptions " +
				"which are excluded from outcome can be viewed  https://vshn.ch[at].#\n",
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
				Cluster:       "TestCluster",
				ExceptionLink: "https://vshn.ch",
				Month:         2,
				SI: []ServiceInstance{
					{
						Namespace:  "dev-namespace",
						Instance:   "postgres-dev",
						TargetSLA:  99.9,
						OutcomeSLA: 99.1,
					},
				},
			},
			expectedPayload: "{\"asciidoc\":\"= SLA Report\\n\\nimage::vshn.png[VSHN Logo,100,54,id=vshn_logo]\\n\\n*" +
				"Version of the document: May 2023* +\\n\\n---\\n\\n[big]#Customer: *TestCustomer* +\\nMonth: *February*" +
				" +\\nCluster: *TestCluster*#\\n\\n---\\n\\n[cols=\\\"Namespace, Instance, SLA Target, SLA Outcome\\\"]" +
				"\\n|===\\n| Namespace| Instance| SLA Target| SLA Outcome\\n\\n|dev-namespace|postgres-dev|99.9%|*99.1%*" +
				"\\n\\n|===\\n\\nNOTE: [small]#The list of exceptions which are excluded from outcome can be viewed  " +
				"https://vshn.ch[at].#\\n\",\"vshn_docgen_id\":\"appcat-sla-report\"}",
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
			assert.Equal(t, []byte(tc.expectedPayload), payload)
		})
	}
}
