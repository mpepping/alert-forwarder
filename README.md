## alert-forwarder

`alert-forwarder` is an alert receiver implementation of Webhook type for Prometheus AlertManager.

`alert-forwarder` forwards alerts from AlertManager to Splunk HEC (HTTP Event Collector).

It also implements `Watchdog` alerts checks to ensure that the entire alerting pipeline is functional.

### Configuration
```
silenced: false
log.level: debug
watchdog.check_interval: 15
watchdog.alert_interval: 7200
watchdog.timeout: 300
event.host: "us-east1-01"
event.sourceType: "prometheus_alerts"
collector.host: "hec.example.com"
collector.protocol: "https"
collector.port: 8088
collector.token: "xxxxxxxx"
```
 - silenced - true or false, if you need to silence all alerts (default false)
 - log.level - debug|info|warn|error (default info)
 - watchdog.check_interval - in seconds, how often to check `Watchdog` pipeline
 - watchdog.alert_interval - in seconds, interval to send broken pipeline alerts
 - watchdog.timeout - in seconds, first alert if `Watchdog` was not received during this time
 - event.host - event host, typically Kubernetes cluster name to identify the source of alerts
 - event.sourceType - event sourcetype
 - collector.host - Splunk HEC host name or IP address
 - collector.protocol - http or https (default https)
 - collector.port - HEC port (default 8088)
 - collector.token - HEC authentication token

### Build

Requirements for building

- Go (version 1.14 or higher)
- [docker](https://www.docker.com/) for image building

A Makefile is provided for building tasks.

```bash
cd $GOPATH/src/alert-forwarder
make build
make install
make image
make push
