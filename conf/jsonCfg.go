package conf

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
)

type Configuration struct {
	LocalNets    []*net.IPNet // e.g. [10.1.0.0/16]
	SearchSuffix string       // e.g. rdvu.example.com
	Peers        []net.Addr   // e.g. [10.0.0.2]
	ZonePrimary  net.Addr     // e.g. 10.1.0.1
	BindAddress  net.Addr     // e.g. 10.1.0.2
	ForwardAll   bool         // whether to pass through all DNS updates to the zone primary, or just rendezvous CNAMEs
}

type parseConfiguration struct {
	LocalNets    []string `json:"localNets"`
	SearchSuffix string   `json:"searchSuffix"`
	Peers        []string `json:"peers"`
	ZonePrimary  string   `json:"zonePrimary"`
	BindAddress  string   `json:"bindAddress"`
	ForwardAll   bool     `json:"forwardAll"`
}

func (pc *parseConfiguration) inhabitConfig(c *Configuration) error {
	c.SearchSuffix = pc.SearchSuffix
	if addr, err := net.ResolveIPAddr("ip", pc.ZonePrimary); err != nil {
		return fmt.Errorf("zone primary address '%v' invalid: %v", pc.ZonePrimary, err)
	} else {
		c.ZonePrimary = addr
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
		if addr, err := net.ResolveIPAddr("ip", peer); err != nil {
			return fmt.Errorf("peer %d with value '%v' invalid: %v", idx, peer, err)
		} else {
			c.Peers = append(c.Peers, addr)
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
