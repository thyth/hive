package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"fmt"
	"net"
)

type CNAMECallback func(proposer net.Addr, name string, target string)
type ACallback func(proposer net.Addr, name string, target net.IP)

type PeerCallbacks struct {
	CNAME CNAMECallback
	A     ACallback
	AAAA  ACallback
}

func StartServer(config *conf.Configuration, key *conf.TsigKey, callbacks *PeerCallbacks) {
	tsig := map[string]string{key.ZoneName: key.Key}

	// run both UDP and TCP, since TCP is usually used for zone transfers
	serverUdp := &dns.Server{
		Addr:       config.BindAddress.String() + ":53",
		Net:        "udp",
		TsigSecret: tsig,
	}
	serverTcp := &dns.Server{
		Addr:       config.BindAddress.String() + ":53",
		Net:        "tcp",
		TsigSecret: tsig,
	}

	go func() {
		if err := serverUdp.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	go func() {
		if err := serverTcp.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	dns.HandleFunc(".", handlerGenerator(config, key, callbacks))
}

func handlerGenerator(config *conf.Configuration, key *conf.TsigKey, callbacks *PeerCallbacks) func(dns.ResponseWriter, *dns.Msg) {
	return func(w dns.ResponseWriter, request *dns.Msg) {
		// TODO check if tsig is valid
		if request.Opcode == dns.OpcodeUpdate {
			// add/delete records
			validZoneUpdate := false
			for _, question := range request.Question {
				if question.Name == config.SearchSuffix &&
					question.Qclass == dns.ClassINET &&
					question.Qtype == dns.TypeSOA {
					validZoneUpdate = true
				}
			}
			if validZoneUpdate {
				for _, authority := range request.Ns {
					proposerHost, _, err := net.SplitHostPort(w.RemoteAddr().String())
					if err != nil {
						continue
					}
					proposer := &net.IPAddr{IP: net.ParseIP(proposerHost)}

					switch authority := authority.(type) {
					case *dns.CNAME:
						if callbacks != nil && callbacks.CNAME != nil {
							callbacks.CNAME(proposer, authority.Hdr.Name, authority.Target)
						}
					case *dns.A:
						if callbacks != nil && callbacks.A != nil {
							callbacks.A(proposer, authority.Hdr.Name, authority.A)
						}
					case *dns.AAAA:
						if callbacks != nil && callbacks.AAAA != nil {
							callbacks.AAAA(proposer, authority.Hdr.Name, authority.AAAA)
						}
					}
				}
			}
		} else if request.Opcode == dns.OpcodeQuery {
			// zone transfers
			for _, question := range request.Question {
				if question.Qclass == dns.ClassINET &&
					question.Qtype == dns.TypeAXFR {
					fmt.Printf("Transfer requested for '%v'\n", question.Name)
					// TODO figure out callback signature and how to reply to transfer
				}
			}
		} else {
			fmt.Println(request.String())
		}
		msg := &dns.Msg{}
		msg.SetReply(request)
		w.WriteMsg(msg)
	}
}
