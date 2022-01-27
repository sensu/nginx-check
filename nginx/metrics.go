package nginx

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
)

type MetricDesc struct {
	help       string
	metricType dto.MetricType
}

var (
	labelHost = "host"
	labelPort = "port"

	line1Regex = regexp.MustCompile("^Active connections: (\\d+)$")
	line3Regex = regexp.MustCompile("^(\\d+)\\s+(\\d+)\\s+(\\d+)$")
	line4Regex = regexp.MustCompile("^Reading:\\s+(\\d+)\\s+Writing:\\s+(\\d+)\\s+Waiting:\\s+(\\d+)$")

	metricHelp = map[string]*MetricDesc{
		"nginx_active":   {"number of active connections", dto.MetricType_GAUGE},
		"nginx_handled":  {"handled connections", dto.MetricType_COUNTER},
		"nginx_accepts":  {"accepted connections", dto.MetricType_COUNTER},
		"nginx_requests": {"handled requests", dto.MetricType_COUNTER},
		"nginx_reading":  {"reading requests", dto.MetricType_GAUGE},
		"nginx_writing":  {"writing requests", dto.MetricType_GAUGE},
		"nginx_waiting":  {"keep-alive connections", dto.MetricType_GAUGE},
	}

	nowFn = time.Now
)

// GetMetrics loads the data from the NGINX status URL and generate Prometheus metrics from
// the content.
func GetMetrics(url string, hostname string, port string, timeout time.Duration) ([]*dto.MetricFamily, error) {
	data, err := loadData(url, timeout)
	if err != nil {
		return nil, err
	}

	return extractMetrics(data, hostname, port)
}

func loadData(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid nginx status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading body content: %s", err.Error())
	}

	return body, nil
}

// extractMetrics parses the metrics from the nginx status page output and returns
// the metrics. The content has the following format:
// Active connections: 1
// server accepts handled requests
// 7 7 91
// Reading: 0 Writing: 1 Waiting: 0
func extractMetrics(content []byte, hostname string, port string) ([]*dto.MetricFamily, error) {
	metrics := make([]*dto.MetricFamily, 0, 7)
	nowMS := nowFn().UnixMilli()

	lines := make([]string, 0, 4)
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}

	// make sure there are 4 lines
	if len(lines) != 4 {
		return nil, fmt.Errorf("4 output lines are expected, got %d", len(lines))
	}

	// First line
	line1Match := line1Regex.FindStringSubmatch(lines[0])
	if len(line1Match) != 2 {
		return nil, fmt.Errorf("unexpected input for line 1: %s", lines[0])
	}

	active, err := strconv.ParseUint(line1Match[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid number of connections: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_active", active, nowMS, hostname, port)

	// Third line
	line3Match := line3Regex.FindStringSubmatch(lines[2])
	if len(line3Match) != 4 {
		return nil, fmt.Errorf("unexpected input for line 3: %s", lines[0])
	}

	accepts, err := strconv.ParseUint(line3Match[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid accepts value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_accepts", accepts, nowMS, hostname, port)

	handled, err := strconv.ParseUint(line3Match[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid handled value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_handled", handled, nowMS, hostname, port)

	requests, err := strconv.ParseUint(line3Match[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid requests value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_requests", requests, nowMS, hostname, port)

	// Fourth line
	line4Match := line4Regex.FindStringSubmatch(lines[3])
	if len(line4Match) != 4 {
		return nil, fmt.Errorf("unexpected input for line 4: %s", lines[0])
	}

	reading, err := strconv.ParseUint(line4Match[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid reading value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_reading", reading, nowMS, hostname, port)

	writing, err := strconv.ParseUint(line4Match[2], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid writing value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_writing", writing, nowMS, hostname, port)

	waiting, err := strconv.ParseUint(line4Match[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid waiting value: %s", err.Error())
	}
	metrics = addNewMetric(metrics, "nginx_waiting", waiting, nowMS, hostname, port)

	return metrics, nil
}

func addNewMetric(metrics []*dto.MetricFamily, metricType string, value uint64, timestampMS int64, hostname string, port string) []*dto.MetricFamily {
	metricDesc := metricHelp[metricType]
	if metricDesc == nil {
		return metrics
	}

	family := &dto.MetricFamily{
		Name:   &metricType,
		Help:   &metricDesc.help,
		Type:   &metricDesc.metricType,
		Metric: []*dto.Metric{},
	}

	if metricDesc.metricType == dto.MetricType_COUNTER {
		addNewCounterMetric(family, float64(value), timestampMS, hostname, port)
	} else if metricDesc.metricType == dto.MetricType_GAUGE {
		addNewGaugeMetric(family, float64(value), timestampMS, hostname, port)
	}
	return append(metrics, family)
}

func addNewCounterMetric(family *dto.MetricFamily, value float64, timestampMS int64, host string, port string) {
	counter := &dto.Metric{
		Label: []*dto.LabelPair{{Name: &labelHost, Value: &host}, {Name: &labelPort, Value: &port}},
		Counter: &dto.Counter{
			Value: &value,
		},
		TimestampMs: &timestampMS,
	}
	family.Metric = append(family.Metric, counter)
}

func addNewGaugeMetric(family *dto.MetricFamily, value float64, timestampMS int64, host string, port string) {
	gauge := &dto.Metric{
		Label: []*dto.LabelPair{{Name: &labelHost, Value: &host}, {Name: &labelPort, Value: &port}},
		Gauge: &dto.Gauge{
			Value: &value,
		},
		TimestampMs: &timestampMS,
	}
	family.Metric = append(family.Metric, gauge)
}
