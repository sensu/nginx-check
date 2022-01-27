package main

import (
	"bytes"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/sensu/nginx-check/nginx"
	"github.com/sensu/sensu-go/types"
	"github.com/sensu/sensu-plugin-sdk/sensu"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	hostname   string
	port       uint32
	statusPath string
	url        string
	timeout    uint32
}

var (
	nginxUrl      string
	nginxHostname string
	nginxPort     string

	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "nginx-check",
			Short:    "Performs on-demand metrics monitoring of NGINX instances",
			Keyspace: "sensu.io/plugins/nginx-check/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		{
			Path:      "hostname",
			Env:       "NGINX_CHECK_HOSTNAME",
			Argument:  "hostname",
			Shorthand: "",
			Default:   "localhost",
			Usage:     "The NGINX hostname",
			Value:     &plugin.hostname,
		}, {
			Path:      "port",
			Env:       "NGINX_CHECK_PORT",
			Argument:  "port",
			Shorthand: "p",
			Default:   uint32(81),
			Usage:     "The NGINX port number",
			Value:     &plugin.port,
		}, {
			Path:      "status-path",
			Env:       "NGINX_CHECK_STATUS_PATH",
			Argument:  "status-path",
			Shorthand: "",
			Default:   "nginx_status",
			Usage:     "The NGINX status path",
			Value:     &plugin.statusPath,
		}, {
			Path:      "url",
			Env:       "NGINX_CHECK_URL",
			Argument:  "url",
			Shorthand: "u",
			Default:   "",
			Usage:     "The NGINX status path URL",
			Value:     &plugin.url,
		}, {
			Path:      "timeout",
			Env:       "NGINX_CHECK_TIMEOUT",
			Argument:  "timeout",
			Shorthand: "t",
			Default:   uint32(10),
			Usage:     "The request timeout in seconds (0 for no timeout)",
			Value:     &plugin.timeout,
		},
	}
)

func main() {
	useStdin := false
	fi, err := os.Stdin.Stat()
	if err != nil {
		fmt.Printf("Error check stdin: %v\n", err)
		panic(err)
	}
	//Check the Mode bitmask for Named Pipe to indicate stdin is connected
	if fi.Mode()&os.ModeNamedPipe != 0 {
		log.Println("using stdin")
		useStdin = true
	}

	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, useStdin)
	check.Execute()
}

func checkArgs(_ *types.Event) (int, error) {
	if plugin.url != "" {
		nginxUrl = strings.TrimSpace(plugin.url)
		parsedUrl, err := url.Parse(nginxUrl)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid url provided: %s", err.Error())
		}
		nginxHostname = parsedUrl.Hostname()
		nginxPort = parsedUrl.Port()
	} else {
		nginxUrl = fmt.Sprintf("http://%s:%d/%s", plugin.hostname, plugin.port, strings.TrimLeft(plugin.statusPath, "/"))
		_, err := url.Parse(nginxUrl)
		if err != nil {
			return sensu.CheckStateCritical, fmt.Errorf("invalid url built from hostname, port and status-path: %s", nginxUrl)
		}
		nginxHostname = plugin.hostname
		nginxPort = strconv.FormatUint(uint64(plugin.port), 10)
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(_ *types.Event) (int, error) {
	metrics, err := nginx.GetMetrics(nginxUrl, nginxHostname, nginxPort, time.Duration(plugin.timeout)*time.Second)
	if err != nil {
		fmt.Printf("error generating nginx metrics: %s", err.Error())
		return sensu.CheckStateCritical, nil
	}
	err = printMetrics(metrics)
	if err != nil {
		fmt.Printf("error printing metrics: %s", err.Error())
		return sensu.CheckStateCritical, nil
	}

	return sensu.CheckStateOK, nil
}

func printMetrics(metrics []*dto.MetricFamily) error {
	var buf bytes.Buffer
	for _, family := range metrics {
		buf.Reset()
		encoder := expfmt.NewEncoder(&buf, expfmt.FmtText)
		err := encoder.Encode(family)
		if err != nil {
			return err
		}

		fmt.Print(buf.String())
	}

	return nil
}
