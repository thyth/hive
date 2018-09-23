package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"net"
	"sync"
	"time"
)

var (
	SigilDeleteIP = net.IPv4(0, 0, 0, 0)
)

type Zone struct {
	sync.Mutex
	Server       net.Addr
	ARecords     map[string]net.IP
	CNAMERecords map[string]string
}

// ReadZoneEntries will zone transfer and look at A and AAAA records.
func ReadZoneEntries(dnsServer net.Addr, key *conf.TsigKey, zone string) (*Zone, error) {
	axfr := &dns.Transfer{
		TsigSecret: map[string]string{
			key.ZoneName: key.Key,
		},
	}
	msg := &dns.Msg{}
	msg.SetAxfr(zone)
	msg.SetTsig(key.ZoneName, key.Algorithm, 300, time.Now().Unix())
	envelopes, err := axfr.In(msg, dnsServer.String()+":53")
	if err != nil {
		return nil, err
	}
	aRecords := map[string]net.IP{}
	cnameRecords := map[string]string{}
	for envelope := range envelopes {
		if envelope.Error != nil {
			return nil, envelope.Error
		}
		records := envelope.RR
		for _, record := range records {
			switch record := record.(type) {
			case *dns.A:
				aRecords[record.Hdr.Name] = record.A
			case *dns.AAAA:
				aRecords[record.Hdr.Name] = record.AAAA
			case *dns.CNAME:
				cnameRecords[record.Hdr.Name] = record.Target
			}
		}
	}
	return &Zone{
		Server:       dnsServer,
		ARecords:     aRecords,
		CNAMERecords: cnameRecords,
	}, nil
}

// MergeZones takes a canonical (i.e. local) zone and supplements it with suggestions that are not yet present in the
// canonical zone. Intended to be applied with suggestions from highest to lowest priority.
func MergeZones(canonical, suggested *Zone) *Zone {
	merged := &Zone{
		Server:       canonical.Server,
		ARecords:     map[string]net.IP{},
		CNAMERecords: map[string]string{},
	}

	// copy in canonical A and CNAME record maps
	canonical.Lock()
	for name, target := range canonical.ARecords {
		merged.ARecords[name] = target
	}
	for name, target := range canonical.CNAMERecords {
		merged.CNAMERecords[name] = target
	}
	canonical.Unlock()

	// iterate over suggested A and CNAME record maps and record only new values
	suggested.Lock()
	for name, target := range suggested.ARecords {
		if _, present := merged.ARecords[name]; !present {
			merged.ARecords[name] = target
		}
	}
	for name, target := range suggested.CNAMERecords {
		if _, present := merged.CNAMERecords[name]; !present {
			merged.CNAMERecords[name] = target
		}
	}
	suggested.Unlock()

	return merged
}

// Produce a pseudo-Zone as a set of operations required to transform Zone to another. Deletions are signified by the
// presence of records that target either the sigil 0.0.0.0 IP (for A/AAAA records) or an empty string CNAME.
func DiffZones(canonical, comparison *Zone) *Zone {
	diff := &Zone{
		Server:       canonical.Server,
		ARecords:     map[string]net.IP{},
		CNAMERecords: map[string]string{},
	}

	canonical.Lock()
	comparison.Lock()

	// iterate over canonical map keys first
	for name, target := range canonical.ARecords {
		comparisonTarget, present := comparison.ARecords[name]
		if !present {
			// insert a deletion record
			diff.ARecords[name] = SigilDeleteIP
		} else if !target.Equal(comparisonTarget) {
			// insert a change record
			diff.ARecords[name] = comparisonTarget
		}
	}
	for name, target := range canonical.CNAMERecords {
		comparisonTarget, present := comparison.CNAMERecords[name]
		if !present {
			// insert a deletion record
			diff.CNAMERecords[name] = ""
		} else if target != comparisonTarget {
			// insert a change record
			diff.CNAMERecords[name] = comparisonTarget
		}
	}
	// iterate over comparison map keys to detect any additions
	for name, target := range comparison.ARecords {
		_, present := canonical.ARecords[name]
		if !present {
			// insert an addition record
			diff.ARecords[name] = target
		}
	}
	for name, target := range comparison.CNAMERecords {
		_, present := canonical.CNAMERecords[name]
		if !present {
			// insert an addition record
			diff.CNAMERecords[name] = target
		}
	}

	canonical.Unlock()
	comparison.Unlock()

	return diff
}
