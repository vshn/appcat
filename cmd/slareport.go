package cmd

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vshn/appcat/pkg/slareport"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// SlareportCMD specifies the cobra command for triggering the maintenance.
var (
	SlareportCMD       = newSlareportCMD()
	customer           string
	date               string
	timeRange          string
	promURL            string
	awssecretaccesskey string
	awsaccesskeyid     string
	endpointURL        string
	bucket             string
	mimirOrg           string
	serviceSlas        map[string]string
	previousMonth      bool
)

func newSlareportCMD() *cobra.Command {

	command := &cobra.Command{
		Use:   "slareport",
		Short: "Generate SLA reports for customers",
		Long: `
Run the sla report for services. This will query a prometheus endpoint, generate the SLA reports and
upload them to an S3 bucket.`,
		RunE: c.runReport,
	}

	now := time.Now()

	viper.AutomaticEnv()
	viper.SetDefault("PROM_URL", "http://localhost:9090/prometheus")
	viper.SetDefault("BUCKET_NAME", "slareports")
	viper.SetDefault("AWS_SECRET_ACCESS_KEY", "minioadmin")
	viper.SetDefault("AWS_ACCESS_KEY_ID", "minioadmin")
	viper.SetDefault("ENDPOINT_URL", "http://localhost:9000")

	command.Flags().StringVar(&customer, "customer", "", "Set the customer name to run against a specific customer")
	command.Flags().StringVar(&date, "date", now.Format(time.RFC3339), "Specify the date from which the SLA should be reported")
	command.Flags().StringVar(&timeRange, "range", "30d", "Range during which the sla should be evaluated")
	command.Flags().StringVar(&promURL, "promurl", viper.GetString("PROM_URL"), "Prometheus URL where the SLA metrics reside. ENV: PROM_URL")
	command.Flags().StringVar(&awssecretaccesskey, "awssecretaccesskey", viper.GetString("AWS_SECRET_ACCESS_KEY"), "AWS secret access key for uploading the reports to S3. ENV: AWS_SECRET_ACCESS_KEY")
	command.Flags().StringVar(&awsaccesskeyid, "awsaccesskeyid", viper.GetString("AWS_ACCESS_KEY_ID"), "AWS key id for uploading the reports to S3. ENV: AWS_ACCESS_KEY_ID")
	command.Flags().StringVar(&endpointURL, "endpointurl", viper.GetString("ENDPOINT_URL"), "S3 endpoint url for uploading the reports. ENV: ENDPOINT_URL")
	command.Flags().StringVar(&bucket, "bucket", viper.GetString("BUCKET_NAME"), "S3 bucketname for uploading the reports. ENV: BUCKET_NAME")
	command.Flags().StringVar(&mimirOrg, "mimirorg", "", "Set the X-Scope-OrgID header for mimir queries")
	command.Flags().StringToStringVar(&serviceSlas, "sla", nil, "Set SLA for each service comma separated. Ex. 's1=sla1,s2=sla2'. Available services: vshnpostgresql")
	command.Flags().BoolVar(&previousMonth, "previousmonth", false, "Run the report for the previous month. Sets the date to the last day of the previous month and range to 30d. This takes precedence over date and range.")

	return command
}

func (c *controller) runReport(cmd *cobra.Command, _ []string) error {

	l := log.FromContext(cmd.Context())

	slas, err := parseServiceSLA(serviceSlas)
	if err != nil {
		return err
	}

	setPreviousMonth()

	metrics, err := slareport.RunQuery(cmd.Context(), promURL, timeRange, date, mimirOrg, slas)
	if err != nil {
		return err
	}

	uploader, err := slareport.NewPDFUploader(cmd.Context(),
		endpointURL,
		bucket,
		awsaccesskeyid,
		awssecretaccesskey,
	)
	if err != nil {
		return err
	}

	parsedDate, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return err
	}

	for customer, instanceMetrics := range metrics {

		l.Info("Rendering PDF for customer", "customer", customer)

		renderer := slareport.SLARenderer{
			Customer:      customer,
			SI:            instanceMetrics,
			Month:         parsedDate.Month(),
			Year:          parsedDate.Year(),
			ExceptionLink: "https://products.vshn.ch/service_levels.html#_exceptions_to_availability_guarantee",
		}

		pdf, err := renderer.GeneratePDF()
		if err != nil {
			return err
		}

		l.Info("Uploading PDF", "customer", customer, "endpoint", endpointURL, "bucket", bucket)
		err = uploader.Upload(cmd.Context(), slareport.PDF{
			Customer: customer,
			Date:     parsedDate,
			PDFData:  pdf,
		})

		if err != nil {
			return err
		}

	}

	return err
}

func parseServiceSLA(slas map[string]string) (map[string]float64, error) {
	m := map[string]float64{}
	for k, sv := range slas {
		fv, err := strconv.ParseFloat(sv, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse float value %s", sv)
		}
		m[k] = fv
	}
	return m, nil
}

func setPreviousMonth() {
	if !previousMonth {
		return
	}

	date = time.Date(time.Now().Year(), time.Now().Month(), 0, 23, 59, 59, 00, time.UTC).Format(time.RFC3339)

	timeRange = "30d"

}
