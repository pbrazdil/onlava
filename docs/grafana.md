# Grafana Dev Integration

`onlava dev` can supervise a local Grafana process alongside the local Victoria observability sidecars. Grafana is dev-only: `onlava run` does not start it.

Generated files live under `.onlava/grafana/` by default:

```text
.onlava/grafana/conf/grafana.ini
.onlava/grafana/provisioning/datasources/onlava.yaml
.onlava/grafana/provisioning/dashboards/onlava.yaml
.onlava/grafana/dashboards/
.onlava/grafana/data/
.onlava/grafana/logs/
.onlava/grafana/plugins/
```

Grafana is provisioned with stable datasource UIDs:

```text
onlava-victoriametrics
onlava-victorialogs
onlava-victoriatraces-jaeger
```

and stable dashboard UIDs:

```text
onlava-dev-overview
onlava-dev-logs
onlava-dev-endpoint
```

Environment controls:

```sh
ONLAVA_DEV_GRAFANA=auto|1|0
ONLAVA_DEV_GRAFANA_DOWNLOAD=1|0
ONLAVA_GRAFANA_BIN=/path/to/grafana
ONLAVA_GRAFANA_VERSION=13.0.1+security-01
ONLAVA_GRAFANA_PORT=3000
ONLAVA_GRAFANA_DIR=.onlava/grafana
ONLAVA_GRAFANA_PLUGINS_PREINSTALL_SYNC=victoriametrics-metrics-datasource@0.24.0,victoriametrics-logs-datasource@0.27.1
```

`auto` is the default. Missing Grafana or Victoria sidecars degrades the Grafana status without stopping the app. `ONLAVA_DEV_GRAFANA=1` makes Grafana required for `onlava dev` startup. When automatic downloads are enabled, the Grafana archive is extracted under `.onlava/grafana/home/grafana-<version>/`; `onlava dev` also writes local ignore markers so downloaded binaries and local state stay out of git.

Default Grafana, Grafana plugin, and Victoria sidecar versions are pinned in `internal/devtools/versions.json`. Environment variables override those pins for local testing.

Reset local Grafana state with:

```sh
rm -rf .onlava/grafana
```
