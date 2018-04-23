package main

import (
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type config struct {
	CcmEndpoint  string
	CcmList      []ccm
	RegionList   []region
	LogCollector string
}

type ccm struct {
	Name string
	Host string
}

type region struct {
	CcoEndpoint           string
	CcoList               []cco
	RabbitEndpointPublic  string
	RabbitEndpointPrivate string
	RabbitList            []rabbit
}

type cco struct {
	Name string
	Host string
}

type rabbit struct {
	Name string
	Host string
}

var verbose bool

func main() {

	me := path.Base(os.Args[0])
	if len(os.Args) < 2 {
		log.Printf("usage: %s location", me)
		os.Exit(1)
	}
	location := os.Args[1]

	verbose = os.Getenv("VERBOSE") != ""

	cfg := config{}
	reg := region{CcoEndpoint: "url"}
	cfg.RegionList = append(cfg.RegionList, reg)

	dec := yaml.NewDecoder(os.Stdin)
	log.Printf("reading config from stdin...")
	if errDec := dec.Decode(&cfg); errDec != nil {
		log.Printf("error decoding config from stdin: %v", errDec)
	}
	log.Printf("reading config from stdin...done")

	result := run(&cfg, location)

	enc := yaml.NewEncoder(os.Stdout)
	log.Printf("writing config to stdout...")
	if errEnc := enc.Encode(&cfg); errEnc != nil {
		log.Printf("error encoding config to stdout: %v", errEnc)
	}
	log.Printf("writing config to stdout...done")

	if !result {
		log.Printf("FAILURE")
		os.Exit(2)
	}
	if verbose {
		log.Printf("SUCCESS")
	}
}

func run(cfg *config, location string) bool {

	switch {
	case location == "browser":
		return runBrowser(cfg, location)
	case strings.HasPrefix(location, "ccm"):
		return runCcm(cfg, location)
	case strings.HasPrefix(location, "cco"):
		return runCco(cfg, location)
	case strings.HasPrefix(location, "rabbit"):
		return runRabbit(cfg, location)
	default:
		log.Printf("bad location: %s", location)
	}

	return false
}

func runBrowser(cfg *config, location string) bool {
	result := true
	if !test(location, "ccm", cfg.CcmEndpoint, ":443") {
		result = false
	}
	if !test(location, "log-collector", cfg.LogCollector, ":8882") {
		result = false
	}
	for _, r := range cfg.RegionList {
		if !test(location, "rabbit-lb-public", r.RabbitEndpointPublic, ":443") {
			result = false
		}
	}
	return result
}

func runCcm(cfg *config, location string) bool {
	result := true
	return result
}

func runCco(cfg *config, location string) bool {
	result := true
	return result
}

func runRabbit(cfg *config, location string) bool {
	result := true
	return result
}

func test(location, target, host, port string) bool {
	endp := host + port
	connected := open(endp)
	if !connected {
		log.Printf("%s: target=%s: failure: %s", location, target, endp)
		return false
	}
	if verbose {
		log.Printf("%s: target=%s: connected: %s", location, target, endp)
	}
	return true
}

func open(addr string) bool {

	timeout := 3 * time.Second

	c, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		c.Close()
		return true
	}

	if verbose {
		log.Printf("connect failure: %s: %v", addr, err)
	}

	return false
}
