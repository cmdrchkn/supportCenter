package main

import (
	"flag"
	"github.com/instaclustr/supportCenter/agent/collector"
	"github.com/sirupsen/logrus"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"golang.org/x/crypto/ssh"
)

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

			err := collector.CollectStats(agent)
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

			err := collector.CollectLogs(agent)
			if err != nil {
				log.Error("Failed to collect logs from " + host)
			}

			completed <- true
		}(host)
	}

	for i := 0; i < len(scTargets.hosts)+len(lcTargets.hosts); i++ {
		<-completed
	}
}
