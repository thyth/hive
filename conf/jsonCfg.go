package conf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
)

type ZonePeer struct {
	Suffix string   // e.g. west.example.com.
	Server net.Addr // e.g. 10.1.0.1
}

type Configuration struct {
	LocalNets    []*net.IPNet // e.g. [10.1.0.0/16]
	LocalZone    *ZonePeer
	SearchSuffix string      // e.g. rdvu.example.com.
	Peers        []*ZonePeer
	BindAddress  net.Addr    // e.g. 10.1.0.2
	TTL          uint32      // record time to live in seconds
}

type parsePeer struct {
	Suffix string `json:"suffix"`
	Server string `json:"server"`
}

type parseConfiguration struct {
	LocalNets    []string     `json:"localNets"`
	LocalZone    *parsePeer   `json:"localZone"`
	SearchSuffix string       `json:"searchSuffix"`
	Peers        []*parsePeer `json:"peers"`
	BindAddress  string       `json:"bindAddress"`
	TTL          uint32       `json:"ttl"`
}

func (pc *parseConfiguration) inhabitConfig(c *Configuration) error {
	c.SearchSuffix = pc.SearchSuffix
	c.TTL = pc.TTL
	if c.TTL < 300 {
		return fmt.Errorf("ttl must be at least 300 seconds but got %d seconds", c.TTL)
	}
	if pc.LocalZone == nil {
		return fmt.Errorf("localZone must be specified")
	}
	c.LocalZone = &ZonePeer{
		Suffix: pc.LocalZone.Suffix,
	}
	if addr, err := net.ResolveIPAddr("ip", pc.LocalZone.Server); err != nil {
		return fmt.Errorf("zone primary address '%v' invalid: %v", pc.LocalZone.Server, err)
	} else {
		c.LocalZone.Server = addr
	}
	if pc.BindAddress != "" {
		if addr, err := net.ResolveIPAddr("ip", pc.BindAddress); err != nil {
			return fmt.Errorf("bind address '%v' invalid: %v", pc.BindAddress, err)
		} else {
			c.BindAddress = addr
		}
	}
	for idx, localNet := range pc.LocalNets {
		if _, netAddr, err := net.ParseCIDR(localNet); err != nil {
			return fmt.Errorf("local net %d with value '%v' invalid: %v", idx, localNet, err)
		} else {
			c.LocalNets = append(c.LocalNets, netAddr)
		}
	}
	for idx, peer := range pc.Peers {
		if addr, err := net.ResolveIPAddr("ip", peer.Server); err != nil {
			return fmt.Errorf("peer %d with value '%v' invalid: %v", idx, peer, err)
		} else {
			c.Peers = append(c.Peers, &ZonePeer{
				Suffix: peer.Suffix,
				Server: addr,
			})
		}
	}
	return nil
}

func (c *Configuration) UnmarshalJSON(b []byte) error {
	parseCfg := &parseConfiguration{}
	if err := json.Unmarshal(b, &parseCfg); err != nil {
		return err
	}
	return parseCfg.inhabitConfig(c)
}

func ParseFile(confFile string) (*Configuration, error) {
	data, err := ioutil.ReadFile(confFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}
	config := &Configuration{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}
