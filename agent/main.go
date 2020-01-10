package main

import (
	"agent/collector"
	"flag"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"golang.org/x/crypto/ssh"
	"path/filepath"
	"time"
)

const timestampPattern = "20060102T150405"
const collectingFolder = "data"

var (
	user = flag.String("l", "", "User to log in as on the remote machine")
	port = flag.Int("p", 22, "Port to connect to on the remote csHost")
)

var log = logrus.New()

func init() {
	log.Formatter = &prefixed.TextFormatter{
		FullTimestamp: true,
	}
}

func main() {
	log.Info("Instaclustr Agent")

	var scTargets HostList
	flag.Var(&scTargets, "sc", "Stats collecting hostname")

	var lcTargets HostList
	flag.Var(&lcTargets, "lc", "Log collecting hostnames")

	flag.Parse()
	validateCommandLineArguments()

	settings := &Settings{
		Logs:  *collector.LogsCollectorDefaultSettings(),
		Stats: *collector.StatsCollectorDefaultSettings(),
	}
	settingsPath := "settings.yml"
	exists, _ := Exists(settingsPath)
	if exists == true {
		log.Info("Loading settings from ", settingsPath, "...")
		err := settings.Load(settingsPath)
		if err != nil {
			log.Warn(err)
		}
	}

	collectingTimestamp := time.Now().UTC().Format(timestampPattern)
	log.Info("Collecting timestamp: ", collectingTimestamp)

	statsCollector := collector.StatsCollector{
		Settings: &settings.Stats,
		Log:      log,
		Path:     filepath.Join(".", collectingFolder, collectingTimestamp),
	}

	logsCollector := collector.LogsCollector{
		Settings: &settings.Logs,
		Log:      log,
		Path:     filepath.Join(".", collectingFolder, collectingTimestamp),
	}

	log.Info("Stats collecting hosts are: ", scTargets.String())
	log.Info("Log collecting hosts are: ", lcTargets.String())

	completed := make(chan bool, len(scTargets.hosts)+len(lcTargets.hosts))

	for _, host := range scTargets.hosts {
		go func(host string) {
			agent := &collector.SSHAgent{}
			agent.SetTarget(host, *port)
			agent.SetConfig(&ssh.ClientConfig{
				User: *user,
				Auth: []ssh.AuthMethod{
					// TODO ask password or search private key
					ssh.Password("qweasd!"),
				},
				// TODO Ask if we need to check host keys
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			})

			err := statsCollector.Collect(agent)
			if err != nil {
				log.Error("Failed to collect logs from " + host)
			}

			completed <- true
		}(host)
	}

	for _, host := range lcTargets.hosts {
		go func(host string) {
			agent := &collector.SSHAgent{}
			agent.SetTarget(host, *port)
			agent.SetConfig(&ssh.ClientConfig{
				User: *user,
				Auth: []ssh.AuthMethod{
					// TODO ask password or search private key
					ssh.Password("qweasd!"),
				},
				// TODO Ask if we need to check host keys
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			})

			err := logsCollector.Collect(agent)
			if err != nil {
				log.Error("Failed to collect logs from " + host)
			}

			completed <- true
		}(host)
	}

	// TODO Add timeout maybe
	for i := 0; i < len(scTargets.hosts)+len(lcTargets.hosts); i++ {
		<-completed
	}
}
