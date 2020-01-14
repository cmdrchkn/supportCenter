package collector

import (
	"errors"
	"github.com/hnakamur/go-scp"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

/*
Constants
*/
const cassandraGCLogFolderName = "logs"
const perm os.FileMode = 0755

/*
Settings
*/
type LogsCollectorSettings struct {
	Cassandra  CassandraSettings  `yaml:"cassandra"`
	Collecting CollectingSettings `yaml:"collecting"`
}

type CassandraSettings struct {
	ConfigPath string `yaml:"config-path"`
	LogPath    string `yaml:"log-path"`
	HomePath   string `yaml:"home-path"`
}

type CollectingSettings struct {
	Configs []string `yaml:"configs"`
	Logs    []string `yaml:"logs"`
}

func LogsCollectorDefaultSettings() *LogsCollectorSettings {
	return &LogsCollectorSettings{
		Cassandra: CassandraSettings{
			ConfigPath: "/etc/cassandra",
			LogPath:    "/var/log/cassandra",
			HomePath:   "/var/lib/cassandra",
		},
		Collecting: CollectingSettings{
			Configs: []string{
				"cassandra.yaml",
				"cassandra-env.sh",
				"jvm.options",
				"logback.xml",
			},
			Logs: []string{
				"system.log",
			},
		},
	}
}

/*
Collector
*/
type LogsCollector struct {
	Settings *LogsCollectorSettings
	Log      *logrus.Logger
	Path     string
}

func (collector *LogsCollector) Collect(agent *SSHAgent) error {
	log := collector.Log.WithFields(logrus.Fields{
		"prefix": "LC " + agent.host,
	})
	log.Info("Logs collecting started")

	err := agent.Connect()
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("Executing nodetool commands...")
	err = collector.collectNodeToolInfo(agent, log)
	if err != nil {
		log.Error(err)
	}
	log.Info("Executing nodetool commands  OK")

	log.Info("Downloading configuration files...")
	err = collector.downloadConfigurationFiles(agent, log)
	if err != nil {
		log.Error(err)
	}
	log.Info("Downloading configuration files  OK")

	log.Info("Downloading log files...")
	err = collector.downloadLogFiles(agent, log)
	if err != nil {
		log.Error(err)
	}
	log.Info("Downloading log files  OK")

	log.Info("Downloading gc log files...")
	err = collector.downloadGCLogFiles(agent, log)
	if err != nil {
		log.Error(err)
	}
	log.Info("Downloading gc log files  OK")

	log.Info("Logs collecting completed")
	return nil
}

func (collector *LogsCollector) downloadConfigurationFiles(agent *SSHAgent, log *logrus.Entry) error {
	dest := filepath.Join(collector.Path, agent.host, "config")
	err := os.MkdirAll(dest, os.ModePerm)
	if err != nil {
		return errors.New("Failed to create folder for configs (" + dest + ")")
	}

	for _, name := range collector.Settings.Collecting.Configs {
		src := filepath.Join(collector.Settings.Cassandra.ConfigPath, name)
		scpAgent := scp.NewSCP(agent.client)
		err = scpAgent.ReceiveFile(src, dest)
		if err != nil {
			log.Warn("Failed to receive config file '" + src + "' (" + err.Error() + ")")
		}
	}

	return nil
}

func (collector *LogsCollector) downloadLogFiles(agent *SSHAgent, log *logrus.Entry) error {
	dest := filepath.Join(collector.Path, agent.host, "logs")
	err := os.MkdirAll(dest, os.ModePerm)
	if err != nil {
		return errors.New("Failed to create folder for logs (" + dest + ")")
	}

	for _, name := range collector.Settings.Collecting.Logs {
		src := filepath.Join(collector.Settings.Cassandra.LogPath, name)
		scpAgent := scp.NewSCP(agent.client)
		err = scpAgent.ReceiveFile(src, dest)
		if err != nil {
			log.Warn("Failed to receive log file '" + src + "' (" + err.Error() + ")")
		}
	}

	return nil
}

func (collector *LogsCollector) downloadGCLogFiles(agent *SSHAgent, log *logrus.Entry) error {
	dest := filepath.Join(collector.Path, agent.host, "gc_logs")
	err := os.MkdirAll(dest, os.ModePerm)
	if err != nil {
		return errors.New("Failed to create folder for logs (" + dest + ")")
	}

	src := filepath.Join(collector.Settings.Cassandra.HomePath, cassandraGCLogFolderName)

	scpAgent := scp.NewSCP(agent.client)
	err = scpAgent.ReceiveDir(src, dest, func(parentDir string, info os.FileInfo) (b bool, err error) {
		// TODO generate gc logs
		log.Info("copy ", parentDir)
		return true, nil
	})
	if err != nil {
		log.Warn("Failed to receive gc log files (" + err.Error() + ")")
	}

	return nil
}

func (collector *LogsCollector) collectNodeToolInfo(agent *SSHAgent, log *logrus.Entry) error {
	commands := [...]string{
		"nodetool info",
		"nodetool version",
		"nodetool status",
		"nodetool tpstats",
		"nodetool compactionstats -H",
		"nodetool gossipinfo",
		"nodetool cfstats -H",
		"nodetool ring",
	}

	path := filepath.Join(collector.Path, agent.host, "nodetool")
	err := os.MkdirAll(path, perm)
	if err != nil {
		return errors.New("Failed to create folder for nodetool info (" + err.Error() + ")")
	}

	for _, command := range commands {
		sout, _, err := agent.ExecuteCommand(command)
		if err != nil {
			log.Error("Failed to execute '" + command + "' (" + err.Error() + ")")
			continue
		}

		fileName := strings.ReplaceAll(command, " ", "_") + ".info"
		err = ioutil.WriteFile(filepath.Join(path, fileName), sout.Bytes(), perm)
		if err != nil {
			log.Error("Failed to save '" + command + "' data (" + err.Error() + ")")
			continue
		}
	}

	return nil
}
