package conf

import (
	"github.com/miekg/dns"

	"encoding/json"
	"fmt"
	"io/ioutil"
)

type TsigKey struct {
	Algorithm string `json:"algorithm"` // e.g. "hmac-sha256."
	Key       string `json:"key"`       // base64 encoded value
	ZoneName  string `json:"zoneName"`  // e.g. "hive."
}

func ParseKeyfile(keyFile string) (*TsigKey, error) {
	data, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key file: %v", err)
	}
	key := &TsigKey{}
	if err := json.Unmarshal(data, &key); err != nil {
		return nil, err
	}

	// validate that the fields are understood
	switch key.Algorithm {
	case dns.HmacMD5:
		fallthrough
	case dns.HmacSHA1:
		fallthrough
	case dns.HmacSHA256:
		fallthrough
	case dns.HmacSHA512:
	default:
		return nil, fmt.Errorf("unknown algorithm '%v' in key file", key.Algorithm)
	}

	if key.Key == "" {
		return nil, fmt.Errorf("no key value present in key file")
	}
	if key.ZoneName == "" {
		return nil, fmt.Errorf("no zone name present in key file")
	}

	return key, nil
}
