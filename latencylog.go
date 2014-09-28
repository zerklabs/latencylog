package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/influxdb/influxdb-go"
	"github.com/zerklabs/auburn/logging"
	"github.com/zerklabs/auburn/utils"
)

type Snapshot struct {
	Duration  float64   `json:"duration"`
	Status    int       `json:"status"`
	End       time.Time `json:"end"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Start     time.Time `json:"start"`
	LocalIP   string    `json:"localip"`
	LocalPort int       `json:"localport"`
	Proto     string    `json:"proto"`
}

var (
	snapshots []*Snapshot
	client    *http.Client
	db        *influxdb.Client

	sourceSite string
	destSite   string
	log        = logging.Log
	doTcp      bool
	doWeb      bool
	doDns      bool
	offline    bool // if no influxdb address is given then TRUE
)

func main() {
	defaultDuration, _ := time.ParseDuration("1m1s")

	var (
		tcpAddress       = flag.String("tcp-address", "", "Target IP:PORT to connect to")
		webAddress       = flag.String("web-address", "", "Target URL to connect to")
		dnsServerAddress = flag.String("dns-server-address", "", "Target DNS server to use")
		resolveAddress   = flag.String("resolve-address", "", "Resource to resolve against the DNS server")
		influxdbAddress  = flag.String("influxdb-address", "", "InfluxDB Host")
		influxDatabase   = flag.String("influxdb-database", "", "InfluxDB Database")
		influxdbUsername = flag.String("influxdb-username", "", "InfluxDB Username")
		influxdbPassword = flag.String("influxdb-password", "", "InfluxDB Password")
		sourceLocation   = flag.String("source-location", "UNKN", "4 character site or region code")
		destLocation     = flag.String("dest-location", "UNKN", "4 character site or region code")
		useTLS           = flag.Bool("enable-tls", false, "Enable TLS")
		duration         = flag.Duration("duration", defaultDuration, "How long to run for (minutes)")
		err              error
	)

	flag.Parse()

	if len(*tcpAddress) == 0 && len(*webAddress) == 0 && len(*dnsServerAddress) == 0 {
		fmt.Println("TCP, Web or DNS address required")

		flag.PrintDefaults()
		return
	}

	if len(*dnsServerAddress) > 0 && len(*resolveAddress) == 0 {
		fmt.Println("DNS Address to resolve required")

		flag.PrintDefaults()
		return
	}

	if len(*tcpAddress) > 0 {
		doTcp = true
	} else if len(*webAddress) > 0 {
		doWeb = true
	} else if len(*dnsServerAddress) > 0 {
		doDns = true
	}

	if len(*influxdbAddress) == 0 {
		fmt.Println("WARNING: Results will not be saved")
		offline = true
		//fmt.Println("InfluxDB Host required")
		//flag.PrintDefaults()
		//return
	} else {
		if len(*influxDatabase) == 0 {
			fmt.Println("InfluxDB Database required")
			flag.PrintDefaults()
			return
		}

		if len(*influxdbUsername) == 0 {
			fmt.Println("InfluxDB Username required")
			flag.PrintDefaults()
			return
		}

		if len(*influxdbPassword) == 0 {
			fmt.Println("InfluxDB Password required")
			flag.PrintDefaults()
			return
		}

		if len(*sourceLocation) == 0 {
			fmt.Println("Source location required")
			flag.PrintDefaults()
			return
		}
		sourceSite = *sourceLocation

		if len(*destLocation) == 0 {
			fmt.Println("Destination location required")
			flag.PrintDefaults()
			return
		}
		destSite = *destLocation

		if !offline {
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}

			if *useTLS {
				client = &http.Client{Transport: tr}
			} else {
				client = &http.Client{}
			}

			influxConfig := &influxdb.ClientConfig{
				Host:       *influxdbAddress,
				Database:   *influxDatabase,
				Username:   *influxdbUsername,
				Password:   *influxdbPassword,
				IsSecure:   *useTLS,
				HttpClient: client,
			}

			db, err = influxdb.NewClient(influxConfig)
			if err != nil {
				log.Panic(err)
				return
			}
		}
	}

	log.Infof("Running for: %s", *duration)

	snapshots = make([]*Snapshot, 0)
	resChan := make(chan *Snapshot)

	tickChan := time.NewTicker(time.Second * 1)
	timeChan := time.NewTimer(*duration).C

	for {
		select {
		case <-tickChan.C:
			if doTcp {
				host, port := utils.ParseNetAddress(*tcpAddress)
				go runTCP(host, port, resChan)
			} else if doWeb {
				go runWeb(*webAddress, resChan)
			} else if doDns {
				go runDns(*dnsServerAddress, *resolveAddress, resChan)
			}
		case res := <-resChan:
			log.Infof("%v, %s:%d", res.Duration, res.Host, res.Port)
			snapshots = append(snapshots, res)
		case <-timeChan:
			tickChan.Stop()
			aggregate(snapshots, *duration)
			return
		}
	}
}

func aggregate(snapshots []*Snapshot, duration time.Duration) {
	if len(snapshots) == 0 {
		return
	}

	series := make([]*influxdb.Series, 0)
	total := float64(0)

	log.Infof("Snapshots: %d", len(snapshots))

	for _, v := range snapshots {
		total += v.Duration
		// don't bother creating a series element if we aren't connected
		// to influxdb
		if !offline {
			series = append(series, &influxdb.Series{
				Name:    fmt.Sprintf("%s.%s.latency", strings.ToUpper(sourceSite), destSite),
				Columns: []string{"source", "destination", "destinationport", "duration", "status", "proto"},
				Points:  [][]interface{}{{v.LocalIP, v.Host, v.Port, v.Duration, v.Status, v.Proto}},
			})
		}
	}

	if len(series) > 0 {
		err := db.WriteSeriesWithTimePrecision(series, influxdb.Microsecond)
		if err != nil {
			log.Error(err)
		}
	}

	avg := total / float64(len(snapshots))
	log.Infof("Average after %s and %d checks: %vms", duration, len(snapshots), avg)
}

func runTCP(host string, port int, resChan chan *Snapshot) {
	snap := &Snapshot{}

	snap.Host = host
	snap.Port = port
	snap.Proto = "TCP"
	snap.Duration = float64(0)
	snap.Status = 0
	snap.Start = time.Now()

	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
	snap.LocalIP, snap.LocalPort = utils.ParseNetAddress(c.LocalAddr().String())

	if err != nil {
		snap.End = time.Now()
		resChan <- snap
		return
	}

	_, err = c.Write(make([]byte, 8))

	if err != nil {
		snap.LocalIP, snap.LocalPort = utils.ParseNetAddress(c.LocalAddr().String())
		snap.End = time.Now()

		resChan <- snap
		return
	}

	snap.End = time.Now()
	dur := snap.End.Sub(snap.Start)

	snap.Duration = dur.Seconds() * 1e3
	snap.LocalIP, snap.LocalPort = utils.ParseNetAddress(c.LocalAddr().String())
	snap.Status = 1

	if err := c.Close(); err != nil {
		snap.Status = 0
		resChan <- snap
		return
	}

	resChan <- snap
	return
}

func runWeb(address string, resChan chan *Snapshot) {
	host := address
	var port int
	snap := &Snapshot{}
	httpClient := http.Client{}

	purl, err := url.Parse(address)
	if err != nil {
		log.Error(err)
	}

	_, port = utils.ParseNetAddress(purl.Host)
	if strings.Contains(purl.Scheme, "https") {
		// set the default https port if not present in the host address
		if port == 0 {
			port = 443
		}
	} else {
		// set the default http port if not present in the host address
		if port == 0 {
			port = 80
		}
	}

	snap.Host = host
	snap.Port = port
	snap.Proto = purl.Scheme
	snap.Duration = float64(0)
	snap.Status = 0
	snap.Start = time.Now()

	resp, err := httpClient.Get(address)
	defer resp.Body.Close()
	if err != nil {
		log.Error(err)
		snap.End = time.Now()
		resChan <- snap
		return
	}

	// finalize the snapshot
	snap.Status = 1
	snap.End = time.Now()
	dur := snap.End.Sub(snap.Start)
	snap.Duration = dur.Seconds() * 1e3
	snap.LocalIP = "127.0.0.1"
	snap.LocalPort = 0
	snap.Status = 1

	resChan <- snap
}

func runDns(dnsServer string, query string, resChan chan *Snapshot) {
	var host string
	var port int
	var snap Snapshot

	host, port = utils.ParseNetAddress(dnsServer)

	snap.Host = host
	snap.Port = port
	snap.Proto = "TCP"
	snap.Duration = float64(0)
	snap.Status = 0
	snap.Start = time.Now()
}
