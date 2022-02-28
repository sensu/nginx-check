# Sensu NGINX Check

## Table of Contents


- [Overview](#overview)
    - [Output Metrics](#output-metrics)
- [Usage Examples](#usage-examples)
    - [Help Output](#help-output)
    - [Environment Variables](#environment-variables)
- [Configuration](#configuration)
    - [Asset Registration](#asset-registration)
    - [Check Definition](#check-definition)
- [Installation from Source](#installation-from-source)
- [Contributing](#contributing)

## Overview

The Sensu NGINX Check is a [Sensu Check][6] that provides access to basic NGINX status information.

### Output Metrics

| Name           | Type    | Description                                                                                                                                   |
|----------------|---------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| nginx_active   | gauge   | The current number of active client connections including Waiting connections.                                                                |
| nginx_accepts  | counter | The total number of accepted client connections.                                                                                              |
| nginx_handled  | counter | The total number of handled connections. Generally, the parameter value is the same as accepts unless some resource limits have been reached. |
| nginx_requests | counter | The total number of client requests.                                                                                                          |
| nginx_reading  | gauge   | The current number of connections where nginx is reading the request header.                                                                  |
| nginx_writing  | gauge   | The current number of connections where nginx is writing the response back to the client.                                                     |
| nginx_waiting  | gauge   | The current number of idle client connections waiting for a request.                                                                          |

## Usage Examples

Default usage, uses `http://localhost:81/nginx_status`:

```shell
nginx-check
```

Specify URL:

```shell
nginx-check --url 'http://localhost:8011/nginx_status' 
```

Specify host, port and status path individually:

```shell
nginx-check --hostname localhost --port 8011 --status-path nginx_status
```

Specify the HTTP timeout in seconds to prevent the plugin from blocking:

```shell
nginx-check --timeout 15
```

Usage:

```shell
nginx-check --help
```

### Help Output

```
Performs on-demand metrics monitoring of NGINX instances

Usage:
  nginx-check [flags]
  nginx-check [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
  -h, --help                 help for nginx-check
      --hostname string      The NGINX hostname (default "localhost")
  -p, --port uint32          The NGINX port number (default 81)
      --status-path string   The NGINX status path (default "nginx_status")
  -t, --timeout uint32       The request timeout in seconds (0 for no timeout) (default 10)
  -u, --url string           The NGINX status path URL

Use "nginx-check [command] --help" for more information about a command.
```

### Environment Variables
| Argument      | Environment Variable       |
|---------------|----------------------------|
| --hostname    | NGINX\_CHECK\_HOSTNAME     |
| --port        | NGINX\_CHECK\_PORT         |
| --status-path | NGINX\_CHECK\_STATUS\_PATH |
| --timeout     | NGINX\_CHECK\_TIMEOUT      |
| --url         | NGINX\_CHECK\_URL          |

## Configuration

### Asset Registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please consider doing so!
If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the following command to add the asset:

```
sensuctl asset add sensu/nginx-check
```

If you're using an earlier version of sensuctl, you can find the asset on
the [Bonsai Asset Index][https://bonsai.sensu.io/assets/sensu/nginx-check].

### Check Definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: nginx-check
  namespace: default
spec:
  command: nginx-check --url 'http://localhost:81/nginx_status' --timeout 10
  subscriptions:
    - system
  runtime_assets:
    - sensu/nginx-check
  output_metric_format: prometheus_text
```

## Installation from Source

The preferred way of installing and deploying this plugin is to use it as an Asset. If you would like to compile and
install the plugin from source or contribute to it, download the latest version or create an executable script from this
source.

From the local path of the nginx-check repository:

```
go build
```

## Contributing

For more information about contributing to this plugin, see [Contributing][1].

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md

[2]: https://github.com/sensu-community/sensu-plugin-sdk

[3]: https://github.com/sensu-plugins/community/blob/master/PLUGIN_STYLEGUIDE.md

[4]: https://github.com/sensu-community/check-plugin-template/blob/master/.github/workflows/release.yml

[5]: https://github.com/sensu-community/check-plugin-template/actions

[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/

[7]: https://github.com/sensu-community/check-plugin-template/blob/master/main.go

[8]: https://bonsai.sensu.io/

[9]: https://github.com/sensu-community/sensu-plugin-tool

[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
