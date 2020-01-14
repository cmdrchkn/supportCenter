package main

import (
	"agent/collector"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const timestampPattern = "20060102T150405"
const collectingRootFolder = "data"
const knownHostsPath = "/.ssh/known_hosts"
const defaultPrivateKeyPath = "/.ssh/id_rsa"

var (
	user              = flag.String("l", "", "User to log in as on the remote machine")
	port              = flag.Int("p", 22, "Port to connect to on the remote host")
	disableKnownHosts = flag.Bool("disable_known_hosts", false, "Skip loading the user’s known-hosts file")

	mcTargets   StringList
	lcTargets   StringList
	privateKeys StringList
)

var log = logrus.New()

func init() {
	log.Formatter = &prefixed.TextFormatter{
		FullTimestamp: true,
	}
}

func init() {
	flag.Var(&mcTargets, "mc", "Metrics collecting hostname")
	flag.Var(&lcTargets, "lc", "Log collecting hostnames")
	flag.Var(&privateKeys, "pk", "List of files from which the identification keys (private key) for public key authentication are read")
}

func main() {
	log.Info("Instaclustr Agent")

	flag.Parse()
	validateCommandLineArguments()

	// Settings
	settings := &Settings{
		Logs:    *collector.LogsCollectorDefaultSettings(),
		Metrics: *collector.MetricsCollectorDefaultSettings(),
	}
	settingsPath := "settings.yml"
	exists, _ := Exists(settingsPath)
	if exists == true {
		log.Info("Loading settings from '", settingsPath, "'...")
		err := settings.Load(settingsPath)
		if err != nil {
			log.Warn(err)
		}
	}

	// SSH Settings
	hostKeyCallback := loadHostKeys()
	authSigners := loadAuthKeys()

	sshConfig := &ssh.ClientConfig{
		User: *user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(authSigners...),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         time.Second * 2,
	}

	// Collecting
	collectingTimestamp := time.Now().UTC().Format(timestampPattern)
	collectingPath := filepath.Join(".", collectingRootFolder, collectingTimestamp)
	log.Info("Collecting timestamp: ", collectingTimestamp)

	metricsCollector := collector.MetricsCollector{
		Settings: &settings.Metrics,
		Logger:   log,
		Path:     collectingPath,
	}

	logsCollector := collector.LogsCollector{
		Settings: &settings.Logs,
		Logger:   log,
		Path:     collectingPath,
	}

	log.Info("Metrics collecting hosts are: ", mcTargets.String())
	log.Info("Log collecting hosts are: ", lcTargets.String())

	completed := make(chan bool, len(mcTargets.items)+len(lcTargets.items))

	for _, host := range mcTargets.items {
		go func(host string) {
			agent := &collector.SSHAgent{}
			agent.SetTarget(host, *port)
			agent.SetConfig(sshConfig)

			err := metricsCollector.Collect(agent)
			if err != nil {
				log.Error("Failed to collect logs from " + host)
			}

			completed <- true
		}(host)
	}

	for _, host := range lcTargets.items {
		go func(host string) {
			agent := &collector.SSHAgent{}
			agent.SetTarget(host, *port)
			agent.SetConfig(sshConfig)

			err := logsCollector.Collect(agent)
			if err != nil {
				log.Error("Failed to collect logs from " + host)
			}

			completed <- true
		}(host)
	}

	// TODO Check errors
	// TODO Add timeout maybe
	for i := 0; i < len(mcTargets.items)+len(lcTargets.items); i++ {
		<-completed
	}

	// Compressing tarball
	log.Info("Compressing collected data (", collectingPath, ")...")
	tarball := filepath.Join(collectingRootFolder, fmt.Sprint(collectingTimestamp, "-data.zip"))
	err := Zip(collectingPath, tarball)
	if err != nil {
		log.Error("Failed to compress collected data (", err, ")")
	} else {
		log.Info("Compressing collected data  OK")
	}

	log.Info("Tarball: ", tarball)
}

func loadHostKeys() ssh.HostKeyCallback {
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if !(*disableKnownHosts) {
		hostsFilePath := filepath.Join(os.Getenv("HOME"), knownHostsPath)

		log.Info("Loading known host '", hostsFilePath, "'...")
		callback, err := knownhosts.New(hostsFilePath)
		if err != nil {
			log.Error("Filed to load known hosts (" + err.Error() + ")")
			os.Exit(1)
		}

		hostKeyCallback = callback
	}
	return hostKeyCallback
}

func loadAuthKeys() []ssh.Signer {
	var signers []ssh.Signer

	keys := []string{
		filepath.Join(os.Getenv("HOME"), defaultPrivateKeyPath),
		// TODO Does somebody use DSA?
		//filepath.Join(os.Getenv("HOME"), "/.ssh/id_dsa"),
	}

	keys = append(keys, privateKeys.items...)

	for _, keyPath := range keys {
		log.Info("Loading private key '", keyPath, "'...")

		key, err := ioutil.ReadFile(keyPath)
		if err != nil {
			log.Warn("Failed to read key '" + keyPath + "' (" + err.Error() + ")")
			continue
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			log.Error("Failed to parse private key '" + keyPath + "' (" + err.Error() + ")")
			continue
		}

		signers = append(signers, signer)
	}

	return signers
}
