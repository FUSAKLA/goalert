package prometheus

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/target/goalert/alert"
)

func TestParseAlertFromBody(t *testing.T) {
	serviceID := "test-service-123"

	tests := []struct {
		name        string
		body        postBody
		expectedErr string
		validate    func(t *testing.T, a *alert.Alert)
	}{
		{
			name: "firing alert with summary",
			body: postBody{
				Status:   "firing",
				GroupKey: "test-group-key",
				CommonLabels: postBodyLabels{
					AlertName: "InstanceDown",
					Instance:  "localhost:9090",
					Severity:  "critical",
				},
				CommonAnnotations: postBodyAnnotations{
					Summary: "Instance localhost:9090 is down",
					Details: "The instance has been down for more than 5 minutes",
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Equal(t, alert.StatusTriggered, a.Status)
				assert.Equal(t, "Instance localhost:9090 is down", a.Summary)
				assert.Contains(t, a.Details, "The instance has been down for more than 5 minutes")
				assert.Equal(t, alert.SeverityCritical, a.Severity)
				assert.Equal(t, alert.SourcePrometheusAlertmanager, a.Source)
				assert.Equal(t, serviceID, a.ServiceID)
				assert.Equal(t, alert.NewUserDedup("test-group-key"), a.Dedup)
			},
		},
		{
			name: "resolved alert",
			body: postBody{
				Status:   "resolved",
				GroupKey: "resolved-group",
				CommonLabels: postBodyLabels{
					AlertName: "InstanceDown",
					Severity:  "warning",
				},
				CommonAnnotations: postBodyAnnotations{
					Summary: "Instance is back up",
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Equal(t, alert.StatusClosed, a.Status)
				assert.Equal(t, "Instance is back up", a.Summary)
				assert.Equal(t, alert.SeverityWarning, a.Severity)
			},
		},
		{
			name: "invalid status",
			body: postBody{
				Status: "pending",
			},
			expectedErr: "invalid status: pending",
		},
		{
			name: "alert with multiple alerts in body",
			body: postBody{
				Status:   "firing",
				GroupKey: "multi-alert",
				CommonLabels: postBodyLabels{
					AlertName: "HighCPU",
				},
				Alerts: []postBodyAlert{
					{
						Labels: postBodyLabels{
							AlertName: "HighCPU",
							Instance:  "server1",
						},
						Annotations: postBodyAnnotations{
							Summary: "CPU usage is high on server1",
						},
					},
					{
						Labels: postBodyLabels{
							AlertName: "HighCPU",
							Instance:  "server2",
						},
						Annotations: postBodyAnnotations{
							Summary: "CPU usage is high on server2",
						},
					},
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Equal(t, alert.StatusTriggered, a.Status)
				// When all alerts have the same AlertName, it combines instances
				assert.Contains(t, a.Summary, "HighCPU")
				assert.Contains(t, a.Summary, "server1,server2")
			},
		},
		{
			name: "unknown severity is preserved",
			body: postBody{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: postBodyLabels{
					AlertName: "TestAlert",
					Severity:  "invalid-severity",
				},
				CommonAnnotations: postBodyAnnotations{
					Summary: "Test alert",
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				// Scan preserves the invalid severity value even though it returns an error
				assert.Equal(t, alert.Severity("invalid-severity"), a.Severity)
			},
		},
		{
			name: "alert with generator URL",
			body: postBody{
				Status:      "firing",
				GroupKey:    "test-group",
				ExternalURL: "http://alertmanager:9093",
				CommonLabels: postBodyLabels{
					AlertName: "TestAlert",
				},
				Alerts: []postBodyAlert{
					{
						Labels: postBodyLabels{
							AlertName: "TestAlert",
							Instance:  "localhost",
						},
						Annotations: postBodyAnnotations{
							Summary: "Test summary",
							Details: "Test details",
						},
						GeneratorURL: "http://prometheus:9090/graph?g0.expr=test",
					},
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Contains(t, a.Details, "[Prometheus Alertmanager UI](http://alertmanager:9093)")
				assert.Contains(t, a.Details, "[View](http://prometheus:9090/graph?g0.expr=test)")
			},
		},
		{
			name: "very long summary gets truncated",
			body: postBody{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: postBodyLabels{
					AlertName: "TestAlert",
				},
				CommonAnnotations: postBodyAnnotations{
					Summary: strings.Repeat("A very long summary ", 100),
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				// Summary should be truncated by SanitizeText
				// The actual length may be slightly different due to sanitization
				assert.LessOrEqual(t, len(a.Summary), alert.MaxSummaryLength+10)
				assert.Greater(t, len(a.Summary), alert.MaxSummaryLength-10)
			},
		},
		{
			name: "alert uses title when summary is missing",
			body: postBody{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: postBodyLabels{
					AlertName: "TestAlert",
				},
				CommonAnnotations: postBodyAnnotations{
					Title: "Alert Title",
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Equal(t, "Alert Title", a.Summary)
			},
		},
		{
			name: "alert uses description when details is missing",
			body: postBody{
				Status:   "firing",
				GroupKey: "test-group",
				CommonLabels: postBodyLabels{
					AlertName: "TestAlert",
				},
				CommonAnnotations: postBodyAnnotations{
					Summary:     "Test Summary",
					Description: "Test Description",
				},
			},
			validate: func(t *testing.T, a *alert.Alert) {
				assert.Contains(t, a.Details, "Test Description")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAlertFromBody(tt.body, serviceID)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				tt.validate(t, result)
			}
		})
	}
}

func TestPostBodySummary(t *testing.T) {
	tests := []struct {
		name     string
		body     postBody
		expected string
	}{
		{
			name: "uses common annotation summary",
			body: postBody{
				CommonAnnotations: postBodyAnnotations{
					Summary: "Common summary",
				},
			},
			expected: "Common summary",
		},
		{
			name: "uses common annotation title when summary missing",
			body: postBody{
				CommonAnnotations: postBodyAnnotations{
					Title: "Common title",
				},
			},
			expected: "Common title",
		},
		{
			name: "uses first alert summary with count for different alerts",
			body: postBody{
				Alerts: []postBodyAlert{
					{
						Labels: postBodyLabels{
							AlertName: "Alert1",
						},
						Annotations: postBodyAnnotations{
							Summary: "First alert summary",
						},
					},
					{
						Labels: postBodyLabels{
							AlertName: "Alert2",
						},
					},
				},
			},
			expected: "First alert summary and 1 others",
		},
		{
			name: "uses common alert name with instance",
			body: postBody{
				CommonLabels: postBodyLabels{
					AlertName: "HighCPU",
					Instance:  "server1",
				},
			},
			expected: "HighCPU server1",
		},
		{
			name: "uses common alert name with multiple instances",
			body: postBody{
				CommonLabels: postBodyLabels{
					AlertName: "HighCPU",
				},
				Alerts: []postBodyAlert{
					{
						Labels: postBodyLabels{
							Instance: "server1",
						},
					},
					{
						Labels: postBodyLabels{
							Instance: "server2",
						},
					},
				},
			},
			expected: "HighCPU server1,server2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.body.Summary()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPostBodyAlertSummary(t *testing.T) {
	tests := []struct {
		name     string
		alert    postBodyAlert
		expected string
	}{
		{
			name: "uses annotation summary",
			alert: postBodyAlert{
				Annotations: postBodyAnnotations{
					Summary: "Alert summary",
				},
			},
			expected: "Alert summary",
		},
		{
			name: "uses annotation title when summary missing",
			alert: postBodyAlert{
				Annotations: postBodyAnnotations{
					Title: "Alert title",
				},
			},
			expected: "Alert title",
		},
		{
			name: "uses alert name and instance when annotations missing",
			alert: postBodyAlert{
				Labels: postBodyLabels{
					AlertName: "InstanceDown",
					Instance:  "localhost:9090",
				},
			},
			expected: "InstanceDown localhost:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.alert.Summary()
			assert.Equal(t, tt.expected, result)
		})
	}
}
