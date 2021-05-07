package main

import (
	"errors"
	"encoding/json"
	"fmt"
	"flag"
	"strconv"
	"net/http"
	"os"
	"time"
	"sync"
	"crypto/tls"
	"io/ioutil"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
	template "github.com/prometheus/alertmanager/template"
	model "github.com/prometheus/common/model"
	hec "github.com/fuyufjh/splunk-hec-go"
)

const (
	CONFIGPATH = "/etc/alert-forwarder-config.yaml"
)

type Configuration struct {
	Silenced              bool      `yaml:"silenced"`
	LogLevel              log.Level `yaml:"log.level"`
	EventHost             string    `yaml:"event.host"`
	EventSourceType       string    `yaml:"event.sourceType"`
	WatchdogCheckInterval int       `yaml:"watchdog.check_interval"`
	WatchdogAlertInterval int       `yaml:"watchdog.alert_interval"`
	WatchdogTimeout       int       `yaml:"watchdog.timeout"`
	CollectorHost         string    `yaml:"collector.host"`
	CollectorProtocol     string    `yaml:"collector.protocol"`
	CollectorPort         int       `yaml:"collector.port"`
	CollectorToken        string    `yaml:"collector.token"`
	LastUpdated           time.Time
}

type responseJSON struct {
	Status  int
	Message string
}

type Watchdog struct {
	sync sync.Mutex
	lastFiringTime time.Time
	lastAlertTime time.Time
	state string
}

var configPath string
var loadedConf, conf *Configuration
var watchdog *Watchdog
var confSync sync.Mutex

func NewConfiguration(filePath string) (*Configuration, error) {
	c := &Configuration{}
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(content, &c)
	if err != nil {
		return c, err
	}
	if c.LogLevel == 0 {
		// default log level is Info
		log.SetLevel(4)
		c.LogLevel = log.GetLevel()
	} else {
		log.SetLevel(c.LogLevel)
	}
	if c.WatchdogCheckInterval == 0 {
		return c, errors.New("configuration: watchdog.check_interval is undefined")
	}
	if c.WatchdogAlertInterval == 0 {
		return c, errors.New("configuration: watchdog.alert_interval is undefined")
	}
	if c.WatchdogTimeout == 0 {
		return c, errors.New("configuration: watchdog.timeout is undefined")
	}
	if c.EventHost == "" {
		return c, errors.New("configuration: event.host is undefined")
	}
	if c.EventSourceType == "" {
		return c, errors.New("configuration: event.sourceType is undefined")
	}
	if c.CollectorHost == "" {
		return c, errors.New("configuration: collector.host is undefined")
	}
	if c.CollectorToken == "" {
		return c, errors.New("configuration: collector.token is undefined")
	}
	if c.CollectorPort == 0 {
		c.CollectorPort = 8088
	}
	if c.CollectorProtocol == "" {
		c.CollectorProtocol = "https"
	}
	if c.CollectorProtocol != "http" && c.CollectorProtocol != "https" {
		return c, errors.New("collector.protocol must be either http or https")
	}
	c.LastUpdated = time.Now()
	return c, nil
}

func reloadConfig() {
	for range time.Tick(time.Second * 15) {
		var fileInfo os.FileInfo
		var err error
		if fileInfo, err = os.Stat(configPath); err != nil {
			log.Fatal("failure to check fileinfo: " + err.Error())
		}
		if fileInfo.ModTime().After(loadedConf.LastUpdated) {
			var newConf *Configuration
			if newConf, err = NewConfiguration(configPath); err != nil {
				log.Errorf("failure to load configuration: " + err.Error())
			} else {
				confSync.Lock()
				loadedConf = newConf
				confSync.Unlock()
				log.Info("reloaded configuration")
			}
		}
	}
}

func asJson(w http.ResponseWriter, status int, message string) {
	data := responseJSON{
		Status:  status,
		Message: message,
	}
	bytes, _ := json.Marshal(data)
	json := string(bytes[:])
	w.WriteHeader(status)
	fmt.Fprint(w, json)
}

func watchdogAlert(status string, startsAt time.Time, endsAt time.Time) (alert *template.Alert) {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	labels["alertname"] = "Watchdog"
	labels["severity"] = "critical"
	annotations["message"] = fmt.Sprintf("Prometheus AlertManager alerting pipeline is not functional. Watchdog alert is not firing for longer than %d minutes", conf.WatchdogTimeout / 60)
	alert = &template.Alert{
		Status: status,
		Labels: labels,
		Annotations: annotations,
		StartsAt: startsAt,
		EndsAt: endsAt,
		Fingerprint: fmt.Sprintf("%016x", model.LabelsToSignature(labels)),
	}
	return
}

func sendToSplunk(alert template.Alert) {
	collectorUrl := conf.CollectorProtocol + "://" + conf.CollectorHost + ":" + strconv.Itoa(conf.CollectorPort)
	client := hec.NewCluster([]string{collectorUrl, collectorUrl}, conf.CollectorToken)
	client.SetHTTPClient(&http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true},}})
	event := hec.NewEvent(alert)
	event.SetTime(time.Now())
	event.SetHost(conf.EventHost)
	event.SetSourceType(conf.EventSourceType)
	data, _ := json.Marshal(event)
	log.Debug(string(data))
	var severity string
	var ok bool
	if severity, ok = alert.Labels["severity"]; !ok {
		severity = "none"
	}
	if conf.Silenced == false {
		if err := client.WriteEvent(event); err == nil {
			log.Infof("(alert %s, severity %s) --> sent to Splunk", alert.Labels["alertname"], severity)
		} else {
			log.Errorf("sendToSplunk(): error: %s", err)
		}
	} else {
		log.Infof("(alert %s, severity %s) --> silenced", alert.Labels["alertname"], severity)
	}
}

func updateWatchdog() {
	watchdog.sync.Lock()
	watchdog.lastFiringTime = time.Now()
	startsAt := watchdog.lastAlertTime
	watchdog.lastAlertTime = time.Now().Add(time.Duration(-conf.WatchdogAlertInterval) * time.Second)
	watchdogState := watchdog.state
	watchdog.state = "firing"
	watchdog.sync.Unlock()
	if watchdogState == "pending" {
		alert := watchdogAlert("resolved", startsAt, time.Now())
		go sendToSplunk(*alert)
	}
}

func checkWatchdog() {
	confSync.Lock()
	conf = loadedConf
	confSync.Unlock()
	for range time.Tick(time.Second * time.Duration(conf.WatchdogCheckInterval)) {
		watchdog.sync.Lock()
		lastFiringTime := watchdog.lastFiringTime
		lastAlertTime := watchdog.lastAlertTime
		watchdog.sync.Unlock()
		if time.Now().After(lastFiringTime.Add(time.Second * time.Duration(conf.WatchdogTimeout))) {
			if time.Now().After(lastAlertTime.Add(time.Second * time.Duration(conf.WatchdogAlertInterval))) {
				watchdog.sync.Lock()
				startsAt := time.Now()
				watchdog.lastAlertTime = startsAt
				watchdog.state = "pending"
				watchdog.sync.Unlock()
				alert := watchdogAlert("firing", startsAt, time.Time{})
				go sendToSplunk(*alert)
			}
		}
	}
}

func webhook(w http.ResponseWriter, r *http.Request) {
	confSync.Lock()
	conf = loadedConf
	confSync.Unlock()
	defer r.Body.Close()
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		asJson(w, http.StatusBadRequest, err.Error())
		log.Errorf("JSON decode error: %s", err.Error())
		return
	}
	for _, alert := range data.Alerts {
		if alertname, ok := alert.Labels["alertname"]; ok {
			if alertname == "Watchdog" {
				updateWatchdog()
				log.Info("processed Watchdog event")
			} else {
				go sendToSplunk(alert)
			}
		}
	}
	asJson(w, http.StatusOK, "success")
}

func healthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok!")
}

func main() {
	var err error
	optConfigPath := flag.String("config", "", "absolute path to the configuration file")
	flag.Parse()
	if len(*optConfigPath) > 0 {
		configPath = *optConfigPath
	} else {
		configPath = CONFIGPATH
	}
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	if loadedConf, err = NewConfiguration(configPath); err != nil {
		log.Fatal("failure to parse configuration: " + err.Error())
	}
	conf = loadedConf
	watchdog = &Watchdog{
		lastFiringTime: time.Now(),
		lastAlertTime: time.Now().Add(time.Duration(-conf.WatchdogAlertInterval) * time.Second),
		state: "firing",
	}
	http.HandleFunc("/healthz", healthz)
	http.HandleFunc("/alerts", webhook)
	var listenAddress string
	if os.Getenv("PORT") != "" {
		listenAddress = ":" + os.Getenv("PORT")
	} else {
		listenAddress = ":8888"
	}
	log.Infof("listening on: %s", listenAddress)
	go checkWatchdog()
	go reloadConfig()
	log.Info("alert pipeline watchdog is running")
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
