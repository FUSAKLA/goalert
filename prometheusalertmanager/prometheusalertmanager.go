package prometheus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/target/goalert/alert"
	"github.com/target/goalert/integrationkey"
	"github.com/target/goalert/permission"
	"github.com/target/goalert/retry"
	"github.com/target/goalert/util/errutil"
	"github.com/target/goalert/util/log"
	"github.com/target/goalert/validation/validate"
)

/* Example payload

```
{
  "receiver": "goalert",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "InstanceDown",
        "code": "200",
        "instance": "127.0.0.1:9090",
        "job": "prometheus",
        "monitor": "codelab-monitor",
        "severity": "critical"
      },
      "annotations": {
        "details": "127.0.0.1:9090 of job prometheus has been down for more than 1 minute.",
        "summary": "Instance 127.0.0.1:9090 down"
      },
      "startsAt": "2020-08-08T14:32:08.326990857Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://pop-os:9090/graph?g0.expr=promhttp_metric_handler_requests_total+%3E+20\u0026g0.tab=1",
      "fingerprint": "791cec13fcba0368"
    },
    {
      "status": "firing",
      "labels": {
        "alertname": "InstanceDown",
        "code": "200",
        "instance": "localhost:9090",
        "job": "prometheus",
        "monitor": "codelab-monitor",
        "severity": "critical"
      },
      "annotations": {
        "details": "localhost:9090 of job prometheus has been down for more than 1 minute.",
        "summary": "Instance localhost:9090 down"
      },
      "startsAt": "2020-08-08T02:21:08.326990857Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "generatorURL": "http://pop-os:9090/graph?g0.expr=promhttp_metric_handler_requests_total+%3E+20\u0026g0.tab=1",
      "fingerprint": "8df98227bdd81384"
    }
  ],
  "groupLabels": {},
  "commonLabels": {
    "alertname": "InstanceDown",
    "code": "200",
    "job": "prometheus",
    "monitor": "codelab-monitor",
    "severity": "critical"
  },
  "commonAnnotations": {},
  "externalURL": "http://pop-os:9093",
  "version": "4",
  "groupKey": "{}:{}",
  "truncatedAlerts": 0
}
```
*/

type postBodyLabels struct {
	Instance  string
	AlertName string `json:"alertname"`
	Severity  string
	Cluster   string
	Locality  string
	Namespace string
}

type postBodyAnnotations struct {
	Title       string
	Summary     string
	Description string
	Details     string
}

type postBody struct {
	Status            string
	ExternalURL       string
	GroupKey          string
	Alerts            []postBodyAlert
	CommonLabels      postBodyLabels
	CommonAnnotations postBodyAnnotations
}

type postBodyAlert struct {
	Labels       postBodyLabels
	Annotations  postBodyAnnotations
	GeneratorURL string
}

func (a postBodyAlert) Summary() string {
	if a.Annotations.Summary != "" {
		return a.Annotations.Summary
	}
	if a.Annotations.Title != "" {
		return a.Annotations.Title
	}

	return a.Labels.AlertName + " " + a.Labels.Instance
}
func (a postBodyAlert) gen() string {
	if a.GeneratorURL == "" {
		return ""
	}

	return fmt.Sprintf(" [View](%s)", a.GeneratorURL)
}
func (a postBodyAlert) Details() string {
	if a.Annotations.Details != "" {
		return a.Annotations.Details + a.gen()
	}
	if a.Annotations.Description != "" {
		return a.Annotations.Description + a.gen()
	}

	return a.Summary() + a.gen()
}
func (b postBody) Summary() string {
	if b.CommonAnnotations.Summary != "" {
		return b.CommonAnnotations.Summary
	}
	if b.CommonAnnotations.Title != "" {
		return b.CommonAnnotations.Title
	}
	if b.CommonLabels.AlertName == "" {
		// different alerts
		return b.Alerts[0].Summary() + fmt.Sprintf(" and %d others", len(b.Alerts)-1)
	}

	// we have a common alert name
	if b.CommonLabels.Instance != "" {
		return b.CommonLabels.AlertName + " " + b.CommonLabels.Instance
	}

	var instances []string
	for _, a := range b.Alerts {
		instances = append(instances, a.Labels.Instance)
	}

	return b.CommonLabels.AlertName + " " + strings.Join(instances, ",")
}

func (b postBody) Details(payload string) string {
	var s strings.Builder
	if b.ExternalURL != "" {
		fmt.Fprintf(&s, "[Prometheus Alertmanager UI](%s)\n\n", b.ExternalURL)
	}
	if b.CommonAnnotations.Details != "" {
		s.WriteString(b.CommonAnnotations.Details + "\n\n")
	} else if b.CommonAnnotations.Description != "" {
		s.WriteString(b.CommonAnnotations.Description + "\n\n")
	} else {
		for _, a := range b.Alerts {
			s.WriteString(a.Details() + "\n\n")
		}
	}
	if payload != "" {
		fmt.Fprintf(&s, "## Payload\n\n```json\n%s\n```\n", payload)
	}
	return s.String()
}

func clientError(w http.ResponseWriter, code int, err error) bool {
	if err == nil {
		return false
	}

	http.Error(w, http.StatusText(code), code)
	return true
}

// ParseAlertFromBody converts a Prometheus Alertmanager webhook payload into a GoAlert alert.
// It validates the status, sanitizes text fields, and sets appropriate alert properties.
func ParseAlertFromBody(body postBody, serviceID string) (*alert.Alert, error) {
	var status alert.Status
	switch body.Status {
	case "firing":
		status = alert.StatusTriggered
	case "resolved":
		status = alert.StatusClosed
	default:
		return nil, fmt.Errorf("invalid status: %s", body.Status)
	}

	var buf bytes.Buffer
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}

	err = json.Indent(&buf, data, "", "  ")
	if err == nil {
		data = buf.Bytes()
	}

	var alertSeverity alert.Severity
	if err := alertSeverity.Scan(body.CommonLabels.Severity); err != nil {
		// Unknown severity defaults to alert.SeverityUnknown (zero value)
	}

	summary := validate.SanitizeText(body.Summary(), alert.MaxSummaryLength)
	details := validate.SanitizeText(body.Details(string(data)), alert.MaxDetailsLength)

	return &alert.Alert{
		Summary:   summary,
		Details:   details,
		Status:    status,
		Severity:  alertSeverity,
		Source:    alert.SourcePrometheusAlertmanager,
		ServiceID: serviceID,
		Dedup:     alert.NewUserDedup(body.GroupKey),
	}, nil
}

func PrometheusAlertmanagerEventsAPI(aDB *alert.Store, intDB *integrationkey.Store) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		err := permission.LimitCheckAny(ctx, permission.Service)
		if errutil.HTTPError(ctx, w, err) {
			return
		}
		serviceID := permission.ServiceID(ctx)

		var body postBody
		var buf bytes.Buffer
		err = json.NewDecoder(io.TeeReader(r.Body, &buf)).Decode(&body)
		if clientError(w, http.StatusBadRequest, err) {
			log.Logf(ctx, "bad request from prometheus alertmanager: %v", err)
			return
		}

		msg, err := ParseAlertFromBody(body, serviceID)
		if err != nil {
			log.Logf(ctx, "bad request from prometheus alertmanager: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		meta := map[string]string{}
		if body.CommonLabels.Cluster != "" {
			meta["cluster"] = body.CommonLabels.Cluster
		}
		if body.CommonLabels.Locality != "" {
			meta["locality"] = body.CommonLabels.Locality
		}
		if body.CommonLabels.Namespace != "" {
			meta["namespace"] = body.CommonLabels.Namespace
		}

		err = retry.DoTemporaryError(func(int) error {
			_, _, err = aDB.CreateOrUpdateWithMeta(ctx, msg, meta)
			return err
		},
			retry.Log(ctx),
			retry.Limit(10),
			retry.FibBackoff(time.Second),
		)
		if errutil.HTTPError(ctx, w, errors.Wrap(err, "create or update alert for prometheus alertmanager")) {
			return
		}
	}
}
