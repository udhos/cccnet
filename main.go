package main

import (
	"log"
	"net"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"gopkg.in/v2/yaml"
)

const version = "0.0"

type config struct {
	Postgres     string
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
	Name                  string
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
var tabMock = map[string]struct{}{}
var result bool

func main() {

	me := path.Base(os.Args[0])
	if len(os.Args) < 2 {
		log.Printf("usage: %s location", me)
		log.Print("location is one of: browser worker ccoName ccmName rabbitName")
		os.Exit(1)
	}
	location := os.Args[1]

	verbose = os.Getenv("VERBOSE") != ""
	dump := os.Getenv("DUMP") != ""
	mock := os.Getenv("MOCK")
	log.Printf("version %s runtime %s VERBOSE=%v DUMP=%v MOCK=%v", version, runtime.Version(), verbose, dump, mock)

	for _, m := range strings.Split(mock, ",") {
		tabMock[m] = struct{}{}
	}

	cfg := config{}

	dec := yaml.NewDecoder(os.Stdin)
	log.Printf("reading config from stdin...")
	if errDec := dec.Decode(&cfg); errDec != nil {
		log.Printf("error decoding config from stdin: %v", errDec)
	}
	log.Printf("reading config from stdin...done")

	run(&cfg, location)

	if dump {
		enc := yaml.NewEncoder(os.Stdout)
		log.Printf("writing config to stdout...")
		if errEnc := enc.Encode(&cfg); errEnc != nil {
			log.Printf("error encoding config to stdout: %v", errEnc)
		}
		log.Printf("writing config to stdout...done")
	}

	if !result {
		log.Printf("FAILURE")
		os.Exit(2)
	}
	if verbose {
		log.Printf("SUCCESS")
	}
}

func run(cfg *config, location string) {

	switch {
	case location == "browser":
		runBrowser(cfg, location)
		return
	case strings.HasPrefix(location, "ccm"):
		runCcm(cfg, location)
		return
	case strings.HasPrefix(location, "cco"):
		runCco(cfg, location)
		return
	case strings.HasPrefix(location, "rabbit"):
		runRabbit(cfg, location)
		return
	case location == "worker":
		runWorker(cfg, location)
		return
	default:
		log.Printf("bad location: %s", location)
	}

	result = false // force failure
}

func runBrowser(cfg *config, location string) {
	result = true
	test(location, "ccm", cfg.CcmEndpoint, ":443")
	test(location, "log-collector", cfg.LogCollector, ":8882")
	for _, r := range cfg.RegionList {
		test(location, r.Name+",rabbit-lb-public", r.RabbitEndpointPublic, ":443")
	}
}

func runRabbit(cfg *config, location string) {
	result = true
	test(location, "ccm", cfg.CcmEndpoint, ":443")
	for _, reg := range cfg.RegionList {
		for _, rab := range reg.RabbitList {
			if rab.Name == location {
				// found this rabbit

				// 1. test CCO LB
				test(location, reg.Name+",cco-lb", reg.CcoEndpoint, ":8443")
				// 2. test rabbit LB
				test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":7789")   // check that 7789 is open, but not needed from rabbit
				test(location, reg.Name+",rabbit-lb-private", reg.RabbitEndpointPrivate, ":7789") // check that 7789 is open, but not needed from rabbit
				test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":7788")
				test(location, reg.Name+",rabbit-lb-private", reg.RabbitEndpointPrivate, ":7788")
				// 3. test other rabbits
				for _, other := range reg.RabbitList {
					test(location, reg.Name+","+other.Name, other.Host, ":7789") // check that 7789 is open, but not needed from rabbit
					test(location, reg.Name+","+other.Name, other.Host, ":7788")
					test(location, reg.Name+","+other.Name, other.Host, ":4369")
					test(location, reg.Name+","+other.Name, other.Host, ":25672")
					test(location, reg.Name+","+other.Name, other.Host, ":22")
				}
				return
			}
		}
	}
	log.Printf("could not find this rabbit: %s", location)
	result = false
}

func runCcm(cfg *config, location string) {
	result = true
	test(location, "postgres", cfg.Postgres, ":5432")
	test(location, "log-collector", cfg.LogCollector, ":4560")
	test(location, "log-collector", cfg.LogCollector, ":8881")
	for _, reg := range cfg.RegionList {
		test(location, reg.Name+",cco-lb", reg.CcoEndpoint, ":8443")
	}
	for _, ccm := range cfg.CcmList {
		test(location, ccm.Name, ccm.Host, ":22")
	}
}

func runCco(cfg *config, location string) {
	result = true
	test(location, "log-collector", cfg.LogCollector, ":4560")
	test(location, "log-collector", cfg.LogCollector, ":8881")
	test(location, "ccm", cfg.CcmEndpoint, ":8443")
	for _, reg := range cfg.RegionList {
		for _, o := range reg.CcoList {
			if o.Name == location {
				// found this cco

				// 1. test rabbit LB
				test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPublic, ":5671")
				test(location, reg.Name+",rabbit-lb-private", reg.RabbitEndpointPrivate, ":5671")

				// 3. test other ccos
				for _, other := range reg.CcoList {
					test(location, reg.Name+","+other.Name, other.Host, ":22")
					test(location, reg.Name+","+other.Name, other.Host, ":5701")
					test(location, reg.Name+","+other.Name, other.Host, ":27017")
					test(location, reg.Name+","+other.Name, other.Host, ":8443")
				}

				return
			}
		}
	}
	log.Printf("could not find this cco: %s", location)
	result = false
}

func runWorker(cfg *config, location string) {
	result = true
	for _, reg := range cfg.RegionList {
		test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPrivate, ":5671")
		test(location, reg.Name+",rabbit-lb-public", reg.RabbitEndpointPrivate, ":7789")
	}
}

func test(location, target, host, port string) {
	if !connect(location, target, host, port) {
		result = false
	}
}

func connect(location, target, host, port string) bool {
	endp := host + port
	if verbose {
		log.Printf("%s: target=%s: opening: %s", location, target, endp)
	}
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

	if _, found := tabMock[addr]; found {
		return true
	}

	timeout := 3 * time.Second

	c, err := net.DialTimeout("tcp", addr, timeout)
	if err == nil {
		c.Close()
		return true
	}

	log.Printf("connect failure: %s: %v", addr, err)

	return false
}
