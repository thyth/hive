package xform

import (
	"github.com/miekg/dns"
	"github.com/thyth/hive/conf"

	"fmt"
)

func StartServer(config *conf.Configuration, key *conf.TsigKey) {
	server := &dns.Server{
		Addr: config.BindAddress.String() + ":53",
		Net: "udp",
		TsigSecret: map[string]string{key.ZoneName: key.Key},
	}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()
	dns.HandleFunc(".", handlerGenerator(config, key))
}

func handlerGenerator(config *conf.Configuration, key *conf.TsigKey) func(dns.ResponseWriter, *dns.Msg) {
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
					switch authority := authority.(type) {
					case *dns.CNAME:
						fmt.Printf("%v proposes %s -> %s\n", w.RemoteAddr(),
							authority.Hdr.Name, authority.Target)
						// TODO callback
					case *dns.A:
						fmt.Printf("%v proposes %s -> %v\n", w.RemoteAddr(),
							authority.Hdr.Name, authority.A)
						// TODO callback
					case *dns.AAAA:
						fmt.Printf("%v proposes %s -> %v\n", w.RemoteAddr(),
							authority.Hdr.Name, authority.AAAA)
						// TODO callback
					}
				}
			}
		} else {
			// TODO AXFRs
			fmt.Println(request.String())
		}
		msg := &dns.Msg{}
		msg.SetReply(request)
		w.WriteMsg(msg)
	}
}
