package main

import (
	"net/http"
	"strconv"

	"github.com/russellcardullo/go-pingdom/pingdom"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "pingdom"
)

var (
	pingdomUp = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Whether the last pingdom scrape was successfull (1: up, 0: down)",
		nil, nil,
	)
	pingdomCheckStatus = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "check_status"),
		"The current status of the check (1: true, 0: false)",
		[]string{"id", "name", "hostname", "status"}, nil,
	)
	pingdomCheckResponseTime = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "check_response_time"),
		"The response time of the last test in milliseconds",
		[]string{"id", "name", "hostname"}, nil,
	)
	pingdomCheckResolution = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "check_resolution"),
		"The resolution of the check",
		[]string{"id", "name", "hostname"}, nil,
	)
)

// Exporter type
type Exporter struct {
	client *pingdom.Client
}

// NewExporter returns a new exporter
func NewExporter(username string, password string, apiKey string) (*Exporter, error) {
	client := pingdom.NewClient(
		username,
		password,
		apiKey,
	)
	return &Exporter{
		client: client,
	}, nil
}

// Describe all the metrics we export
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- pingdomUp
	ch <- pingdomCheckStatus
	ch <- pingdomCheckResponseTime
	ch <- pingdomCheckResolution
}

// Collect metrics from the pingdom API
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {

	checks, err := e.client.Checks.List()
	if err != nil {
		log.Errorln("Error retrieving checks", err)
		ch <- prometheus.MustNewConstMetric(
			pingdomUp, prometheus.GaugeValue, 0,
		)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		pingdomUp, prometheus.GaugeValue, 1,
	)

	for _, check := range checks {
		id := strconv.Itoa(check.ID)

		unknown := float64(0)
		paused := float64(0)
		up := float64(0)
		unconfirmedDown := float64(0)
		down := float64(0)

		switch check.Status {
		case "unknown":
			unknown = float64(1)
		case "paused":
			paused = float64(1)
		case "up":
			up = float64(1)
		case "unconfirmed_down":
			unconfirmedDown = float64(1)
		case "down":
			down = float64(1)
		}

		// pingdomCheckStatus
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckStatus, prometheus.GaugeValue, unknown, id, check.Name, check.Hostname, "unknown",
		)
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckStatus, prometheus.GaugeValue, paused, id, check.Name, check.Hostname, "paused",
		)
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckStatus, prometheus.GaugeValue, up, id, check.Name, check.Hostname, "up",
		)
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckStatus, prometheus.GaugeValue, unconfirmedDown, id, check.Name, check.Hostname, "unconfirmed_down",
		)
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckStatus, prometheus.GaugeValue, down, id, check.Name, check.Hostname, "down",
		)

		// pingdomCheckResponseTime
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckResponseTime, prometheus.GaugeValue, float64(check.LastResponseTime), id, check.Name, check.Hostname,
		)

		// pingdomCheckResolution
		ch <- prometheus.MustNewConstMetric(
			pingdomCheckResolution, prometheus.GaugeValue, float64(check.Resolution), id, check.Name, check.Hostname,
		)
	}

}

func init() {
	prometheus.MustRegister(version.NewCollector(namespace + "_exporter"))
}

func main() {
	var (
		listenAddress = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":8000").String()
		metricsPath   = kingpin.Flag("web.metrics-path", "Path under which to expose metrics").Default("/metrics").String()
		server        = kingpin.Command("server", "")
		username      = server.Arg("pingdom.username", "Username for the Pingdom account").Required().String()
		password      = server.Arg("pingdom.password", "Password for the Pingdom account").Required().String()
		apiKey        = server.Arg("pingdom.key", "API key").Required().String()
	)

	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print(namespace + "_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	exporter, err := NewExporter(*username, *password, *apiKey)
	if err != nil {
		log.Fatalln("Error")
	}

	prometheus.MustRegister(exporter)

	log.Infoln("Starting "+namespace+"_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
						 <head><title>Pingdom Exporter</title></head>
						 <body>
						 <h1>Pingdom Exporter</h1>
						 <p><a href='` + *metricsPath + `'>Metrics</a></p>
						 </body>
						 </html>`))
	})

	log.Infoln("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
