package main

import (
	"github.com/miekg/dns"

	"github.com/thyth/hive/conf"
	"github.com/thyth/hive/xform"

	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
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

	var primaryZone *xform.Zone
	var rendezvousZone *xform.Zone
	peerZones := make([]*xform.Zone, len(config.Peers))
	// the defaultZone is populated by update requests not associated with configured peers, whose values are merged
	// into the rendezvous zone at lowest priority (i.e. any peer configured value will take precedence).
	defaultZone := &xform.Zone{
		ARecords:     map[string]net.IP{},
		CNAMERecords: map[string]string{},
	}
	zoneByServer := map[string]*xform.Zone{}
	zoneByName := map[string]*xform.Zone{}

	updateByZoneMutex := &sync.Mutex{}
	updateByZone := map[*xform.Zone]uint32{}

	zoneUpdateMutex := &sync.Mutex{}
	localZoneUpdate := func() {
		zoneUpdateMutex.Lock()
		defer zoneUpdateMutex.Unlock()
		// A) merge zones starting with the primary zone, through the peers in priority order, followed by the default
		//    zone to determine the new rendezvous zone state
		merged := tranposePrimary(primaryZone, config)
		for idx, peer := range peerZones {
			tranposed := tranposePeer(peer, config.Peers[idx].Suffix, config.SearchSuffix)
			merged = xform.MergeZones(merged, tranposed)
		}
		merged = xform.MergeZones(merged, defaultZone)
		// B) diff the new rendezvous zone with the existing one, and update the primary zone
		diff := xform.DiffZones(rendezvousZone, merged)
		if len(diff.CNAMERecords) > 0 {
			// there is at least one record different... update CNAMEs in the rendezvous record
			updateByZoneMutex.Lock()
			updateByZone[defaultZone]++
			updateByZoneMutex.Unlock()

			// write updates
			for name, target := range diff.CNAMERecords {
				if err := xform.WriteUpdate(config.LocalZone.Server, config, key, &xform.Mapping{
					Name:   name,
					Target: target,
				}); err != nil {
					fmt.Printf("Error writing update to rendezvous zone: %v\n", err)
					return
				}
			}

			// track the newly created record as the state of the rendezvous zone
			rendezvousZone = merged
		}
	}

	// Operational sequence:
	// 1) Zone transfer from the local primary DNS server to populate transient cache (no persistent caching in Hive)
	primaryZone, err = xform.ReadZoneEntries(config.LocalZone.Server, key, config.LocalZone.Suffix)
	if err != nil {
		fmt.Printf("Zone transfer from primary failed: %v\n", err)
		os.Exit(1)
	}
	rendezvousZone, err = xform.ReadZoneEntries(config.LocalZone.Server, key, config.SearchSuffix)
	initialUpdateRequired := false
	if err != nil {
		fmt.Printf("Initializing new rendezvous zone; transfer from primary failed: %v\n", err)
		rendezvousZone = &xform.Zone{
			Server:       config.LocalZone.Server,
			ARecords:     map[string]net.IP{},
			CNAMERecords: map[string]string{},
		}
		initialUpdateRequired = true
	}
	zoneByServer[config.LocalZone.Server.String()] = primaryZone
	zoneByName[config.LocalZone.Suffix] = primaryZone

	// 2) Zone transfer from all peers and augment transient structures
	for idx, peer := range config.Peers {
		zone, err := xform.ReadZoneEntries(peer.Server, key, peer.Suffix)
		if err != nil {
			peerZones[idx] = zone
		} else {
			fmt.Printf("Unable to transfer zone from peer %v: %v\n", peer.Server, err)
			// store a blank zone -- the peer may not be online yet
			peerZones[idx] = &xform.Zone{
				Server:       peer.Server,
				ARecords:     map[string]net.IP{},
				CNAMERecords: map[string]string{},
			}
		}
		zoneByServer[peer.Server.String()] = peerZones[idx]
		zoneByName[peer.Suffix] = peerZones[idx]
	}
	// 3) Start listening for DNS update requests from peers (and/or DHCP servers)
	xform.StartServer(config, key, &xform.PeerCallbacks{
		CNAME: func(proposer net.Addr, name string, target string) {
			fmt.Printf("%v proposed '%s' CNAME '%s'\n", proposer, name, target)
			zone, present := zoneByServer[proposer.String()]
			if !present {
				zone = defaultZone
			}
			runUpdate := false

			zone.Lock()
			if zone.CNAMERecords[name] != target {
				zone.CNAMERecords[name] = target
				updateByZoneMutex.Lock()
				updateByZone[zone]++
				updateByZoneMutex.Unlock()
				runUpdate = true
			}
			zone.Unlock()
			if runUpdate {
				localZoneUpdate()
			}
		},
		A: func(proposer net.Addr, name string, target net.IP) {
			fmt.Printf("%v proposed '%s' A '%v'\n", proposer, name, target)
			zone, present := zoneByServer[proposer.String()]
			if !present {
				zone = defaultZone
			}
			runUpdate := false

			zone.Lock()
			if !zone.ARecords[name].Equal(target) {
				zone.ARecords[name] = target
				updateByZoneMutex.Lock()
				updateByZone[zone]++
				updateByZoneMutex.Unlock()
				runUpdate = true
			}
			zone.Unlock()
			if runUpdate {
				localZoneUpdate()
			}
		},
		AAAA: func(proposer net.Addr, name string, target net.IP) {
			fmt.Printf("%v proposed '%s' AAAA '%v'\n", proposer, name, target)
			zone, present := zoneByServer[proposer.String()]
			if !present {
				zone = defaultZone
			}
			runUpdate := false

			zone.Lock()
			if !zone.ARecords[name].Equal(target) {
				zone.ARecords[name] = target
				updateByZoneMutex.Lock()
				updateByZone[zone]++
				updateByZoneMutex.Unlock()
				runUpdate = true
			}
			zone.Unlock()
			if runUpdate {
				localZoneUpdate()
			}
		},
		Serial: func(zoneName string) uint32 {
			// similar to RFC1912 (which presents an ISO 8601 date followed by a 2 digit revision number), this process
			// uses a 2 digit year instead of a 4 digit year, so the revision number may be 4 digits. This similarly
			// should guarantee monotonic increases, except on century crossings. Be sure to restart your hive on
			// January 1st, 2100, and all subsequent century crossings.
			zone, present := zoneByName[zoneName]
			if !present {
				zone = defaultZone
			}
			updateByZoneMutex.Lock()
			index := updateByZone[zone]
			updateByZoneMutex.Unlock()

			if index >= 10000 {
				index = 10000 - 1
			}

			now := time.Now()
			dateIndex := now.Day() + int(now.Month())*100 + (now.Year()%100)*10000

			return uint32(dateIndex)*10000 + index
		},
		Transfer: func(zoneName string) []*xform.Mapping {
			zone, present := zoneByName[zoneName]
			if !present {
				zone = rendezvousZone
			}
			var mappings []*xform.Mapping
			zone.Lock()
			for name, target := range zone.CNAMERecords {
				mappings = append(mappings, &xform.Mapping{
					Name:   name,
					Target: target,
				})
			}
			for name, address := range zone.ARecords {
				mappings = append(mappings, &xform.Mapping{
					Name: name,
					IP:   address,
				})
			}
			zone.Unlock()
			return mappings
		},
	})

	if initialUpdateRequired {
		localZoneUpdate()
	}

	select {}
}

func tranposePrimary(zone *xform.Zone, config *conf.Configuration) *xform.Zone {
	if zone == nil || config == nil {
		return nil
	}
	// tranpose A/AAAA records into CNAME records to the rendezvous suffix
	zone.Lock()
	tranposed := &xform.Zone{
		Server:       zone.Server,
		ARecords:     map[string]net.IP{},
		CNAMERecords: map[string]string{},
	}

	for name, target := range zone.ARecords {
		tranposedName := ""
		if !dns.IsSubDomain(config.LocalZone.Suffix, name) {
			continue
		} else {
			tranposedName = strings.TrimSuffix(name, config.LocalZone.Suffix) + config.SearchSuffix
			tranposedName = strings.ToLower(tranposedName)
		}
		for _, localNet := range config.LocalNets {
			if localNet.Contains(target) {
				tranposed.CNAMERecords[tranposedName] = strings.ToLower(name)
				break
			}
		}
	}
	zone.Unlock()
	return tranposed
}

func tranposePeer(zone *xform.Zone, peerSuffix, rendezvousSuffix string) *xform.Zone {
	// tranpose A/AAAA records into CNAME records to the rendezvous suffix
	zone.Lock()
	tranposed := &xform.Zone{
		Server:       zone.Server,
		ARecords:     map[string]net.IP{},
		CNAMERecords: map[string]string{},
	}

	for name := range zone.ARecords {
		tranposedName := ""
		if !dns.IsSubDomain(peerSuffix, name) {
			continue
		} else {
			tranposedName = strings.TrimSuffix(name, peerSuffix) + rendezvousSuffix
			tranposedName = strings.ToLower(tranposedName)
		}
		tranposed.CNAMERecords[tranposedName] = strings.ToLower(name)
	}
	zone.Unlock()
	return tranposed
}
