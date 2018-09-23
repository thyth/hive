package main

import (
	"github.com/thyth/hive/conf"

	"flag"
	"fmt"
	"os"
)

func main() {
	configFile := ""
	dnsKeyFile := ""

	flag.StringVar(&configFile, "config", "", "Path to a JSON configuration file")
	flag.StringVar(&dnsKeyFile, "key", "", "Path to a DNS key file")

	flag.Parse()
	if configFile == "" || dnsKeyFile == "" {
		flag.Usage()
		return
	}

	config, err := conf.ParseFile(configFile)
	if err != nil {
		fmt.Printf("Error processing config file: %v\n", err)
		os.Exit(1)
	}

	key, err := conf.ParseKeyfile(dnsKeyFile)
	if err != nil {
		fmt.Printf("Error processing key file: %v\n", err)
		os.Exit(1)
	}

	// Operational sequence:
	// 1) Zone transfer from the zone primary DNS server to populate transient cache (no persistent caching in Hive)
	// 2) Start listening for DNS update requests from peers (and/or DHCP servers)
	// 3) Zone transfer from all peers to augment transient structures
	// 4) Clean up any stale rendezvous records
	// 5) Prioritize A/AAAA records for local networks, and update CNAME rendezvous records in zone primary server
	// 6) Continue to update rendezvous records as updates arrive
	// TODO all of the 6 steps above
}
