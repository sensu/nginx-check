package nginx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	outputLine1Ok = "Active connections: 33"
	outputLine2Ok = "server accepts handled requests"
	outputLine3Ok = "1237 89 91"
	outputLine4Ok = "Reading: 55 Writing: 767 Waiting: 1234"
	allLinesOk    = outputLine1Ok + "\n" + outputLine2Ok + "\n" + outputLine3Ok + "\n" + outputLine4Ok + "\n"
)

var (
	hostname = "myhost.sensu.local"
	port     = "3456"

	activeValue   = float64(33)
	handledValue  = float64(89)
	acceptsValue  = float64(1237)
	requestsValue = float64(91)
	readingValue  = float64(55)
	writingValue  = float64(767)
	waitingValue  = float64(1234)

	expectedValues = []*struct {
		key   string
		value *float64
	}{
		{"nginx_active", &activeValue},
		{"nginx_handled", &handledValue},
		{"nginx_accepts", &acceptsValue},
		{"nginx_requests", &requestsValue},
		{"nginx_reading", &readingValue},
		{"nginx_writing", &writingValue},
		{"nginx_waiting", &waitingValue},
	}
)

func TestLoadData(t *testing.T) {
	testServer := createTestServer()
	defer testServer.Close()

	t.Run("happy scenario", func(t *testing.T) {
		data, err := loadData(testServer.URL+"/nginx_status", 10*time.Second)
		require.NoError(t, err)
		assert.Equal(t, allLinesOk, string(data))
	})

	t.Run("invalid url (404 error)", func(t *testing.T) {
		data, err := loadData(testServer.URL+"/seriously", 10*time.Second)
		require.Error(t, err)
		require.Nil(t, data)
	})

	t.Run("invalid host/port", func(t *testing.T) {
		data, err := loadData("http://127.0.0.1:33333", 10*time.Second)
		require.Error(t, err)
		require.Nil(t, data)
	})

	t.Run("request timeout", func(t *testing.T) {
		start := time.Now()
		data, err := loadData(testServer.URL+"/sleep", 1*time.Second)
		timeDelta := time.Since(start)
		// the /sleep url waits 10 seconds before returning. anything bellow 8 seconds is fine
		assert.Error(t, err)
		assert.Nil(t, data)
		assert.Less(t, int64(timeDelta), int64(8*time.Second))
	})
}

func TestExtractMetrics(t *testing.T) {
	now := time.Now()
	nowMS := now.UnixMilli()
	nowFn = func() time.Time { return now }

	expectedMetrics := make(map[string]*dto.MetricFamily, len(expectedValues))
	for _, value := range expectedValues {
		details := metricHelp[value.key]
		dtoMetric := dto.Metric{
			TimestampMs: &nowMS,
			Label:       []*dto.LabelPair{{Name: &labelHost, Value: &hostname}, {Name: &labelPort, Value: &port}},
		}
		switch details.metricType {
		case dto.MetricType_COUNTER:
			dtoMetric.Counter = &dto.Counter{Value: value.value}
		case dto.MetricType_GAUGE:
			dtoMetric.Gauge = &dto.Gauge{Value: value.value}
		}

		expectedMetrics[value.key] = &dto.MetricFamily{
			Name:   &value.key,
			Help:   &details.help,
			Type:   &details.metricType,
			Metric: []*dto.Metric{&dtoMetric},
		}
	}

	testCases := []struct {
		name            string
		lines           []byte
		expectedError   string
		expectedMetrics map[string]*dto.MetricFamily
	}{
		{
			name:            "happy output",
			lines:           []byte(allLinesOk),
			expectedError:   "",
			expectedMetrics: expectedMetrics,
		}, {
			name:            "happy output with spaces",
			lines:           []byte("   " + outputLine1Ok + "  \n  " + outputLine2Ok + " \n  " + outputLine3Ok + "   \n  " + outputLine4Ok + "  \n"),
			expectedError:   "",
			expectedMetrics: expectedMetrics,
		}, {
			name:            "invalid line 1",
			lines:           []byte(outputLine1Ok + "aaa\n" + outputLine2Ok + "\n" + outputLine3Ok + "\n" + outputLine4Ok + "\n"),
			expectedError:   "unexpected input for line 1",
			expectedMetrics: nil,
		}, {
			name:            "invalid line 3",
			lines:           []byte(outputLine1Ok + "\n" + outputLine2Ok + "\nhuh" + outputLine3Ok + "\n" + outputLine4Ok + "\n"),
			expectedError:   "unexpected input for line 3",
			expectedMetrics: nil,
		}, {
			name:            "invalid line 4",
			lines:           []byte(outputLine1Ok + "\n" + outputLine2Ok + "\n" + outputLine3Ok + "\nhuh" + outputLine4Ok + "\n"),
			expectedError:   "unexpected input for line 4",
			expectedMetrics: nil,
		}, {
			name:            "one line missing",
			lines:           []byte(outputLine1Ok + "\n" + outputLine2Ok + "\n" + outputLine3Ok + "\n"),
			expectedError:   "4 output lines are expected, got 3",
			expectedMetrics: nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			metrics, err := extractMetrics(test.lines, hostname, port)
			if test.expectedError == "" {
				require.NoError(t, err)
				//for _, metric := range
				//require.True(t, equalMetricFamilies(metrics, test.expectedMetrics))
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expectedError)
			}

			if len(test.expectedMetrics) > 0 {
				assert.Equal(t, len(expectedMetrics), len(metrics))
				for _, metric := range metrics {
					expectedMetric := expectedMetrics[metric.GetName()]
					assertEqualMetricFamilies(t, expectedMetric, metric)
				}
			}
		})
	}
}

func createTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.RequestURI, "nginx_status") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(allLinesOk))
		} else if strings.HasSuffix(r.RequestURI, "sleep") {
			time.Sleep(10 * time.Second)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(allLinesOk))
		} else {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("NOT FOUND"))
		}
	}))
}

func assertEqualMetricFamilies(t *testing.T, expected *dto.MetricFamily, actual *dto.MetricFamily) bool {
	if expected.GetName() != actual.GetName() {
		t.Errorf("name not matching: expected=%s; actual=%s", expected.GetName(), actual.GetName())
		return false
	}
	if expected.GetHelp() != actual.GetHelp() {
		t.Errorf("help not matching: expected=%s; actual=%s", expected.GetHelp(), actual.GetHelp())
		return false
	}
	if expected.GetType() != actual.GetType() {
		t.Errorf("type not matching: expected=%s; actual=%s", expected.GetType(), actual.GetType())
		return false
	}
	if len(expected.Metric) != 1 {
		t.Errorf("one expected metric required: received %d", len(expected.Metric))
		return false
	}
	if len(actual.Metric) != 1 {
		t.Errorf("one actual metric required: received %d", len(actual.Metric))
		return false
	}
	if expected.Metric[0].GetTimestampMs() != actual.Metric[0].GetTimestampMs() {
		t.Errorf("timestampMs not matching: expected: %d; actual: %d", expected.Metric[0].GetTimestampMs(),
			actual.Metric[0].GetTimestampMs())
		return false
	}
	if *expected.Type == dto.MetricType_COUNTER {
		if expected.Metric[0].Counter.GetValue() != actual.Metric[0].Counter.GetValue() {
			t.Errorf("counter value not matching: expected: %f; actual: %f", expected.Metric[0].Counter.GetValue(),
				actual.Metric[0].Counter.GetValue())
			return false
		}
	} else if *expected.Type == dto.MetricType_GAUGE {
		if expected.Metric[0].Gauge.GetValue() != actual.Metric[0].Gauge.GetValue() {
			t.Errorf("gauge value not matching: expected: %f; actual: %f", expected.Metric[0].Gauge.GetValue(),
				actual.Metric[0].Gauge.GetValue())
			return false
		}
	}

	return true
}
