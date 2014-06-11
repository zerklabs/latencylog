package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/influxdb/influxdb-go"
	h "github.com/zerklabs/gohelpers"
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

	httpUri = ""
	useHttp = false

	client *http.Client
	db     *influxdb.Client
)

func main() {
	var (
		tcpAddress       = flag.String("tcp-address", "", "Target IP:PORT to connect to")
		influxdbAddress  = flag.String("influxdb-address", "", "InfluxDB Host")
		influxDatabase   = flag.String("influxdb-database", "", "InfluxDB Database")
		influxdbUsername = flag.String("influxdb-username", "", "InfluxDB Username")
		influxdbPassword = flag.String("influxdb-password", "", "InfluxDB Password")
		duration         = flag.Int("duration", 1, "How long to run for (minutes)")
		err              error
	)

	flag.Parse()

	if len(*tcpAddress) == 0 {
		fmt.Println("TCP address required")

		flag.PrintDefaults()
		return
	}

	if len(*influxdbAddress) == 0 {
		fmt.Println("InfluxDB Host required")
		flag.PrintDefaults()
		return
	}

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

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client = &http.Client{Transport: tr}

	influxConfig := &influxdb.ClientConfig{
		Host:       *influxdbAddress,
		Database:   *influxDatabase,
		Username:   *influxdbUsername,
		Password:   *influxdbPassword,
		IsSecure:   true,
		HttpClient: client,
	}

	db, err = influxdb.NewClient(influxConfig)

	if err != nil {
		panic(err)
	}

	runFor, _ := time.ParseDuration(fmt.Sprintf("%dm", *duration))
	log.Printf("Running for: %s", runFor)

	snapshots = make([]*Snapshot, 0)
	resChan := make(chan *Snapshot)

	tickChan := time.NewTicker(time.Second * 1)
	timeChan := time.NewTimer(runFor).C

	for {
		select {
		case <-tickChan.C:
			host, port := h.ParseNetAddress(*tcpAddress)
			go runTCP(host, port, resChan)
		case res := <-resChan:
			log.Printf("%v, %s:%d", res.Duration, res.Host, res.Port)
			snapshots = append(snapshots, res)
		case <-timeChan:
			tickChan.Stop()
			aggregate(snapshots, *duration)
			return
		}
	}
}

func aggregate(snapshots []*Snapshot, duration int) {
	if len(snapshots) > 0 {
		series := make([]*influxdb.Series, 0)
		total := float64(0)
		log.Printf("Snapshots: %d", len(snapshots))

		for _, v := range snapshots {
			total += v.Duration
			series = append(series, &influxdb.Series{
				Name:    "measurements",
				Columns: []string{"source", "sourceport", "destination", "destinationport", "duration", "status", "start", "end", "proto"},
				Points:  [][]interface{}{{v.LocalIP, v.LocalPort, v.Host, v.Port, v.Duration, v.Status, v.Start, v.End, v.Proto}},
			})
		}

		avg := total / float64(len(snapshots))
		log.Printf("Average after %d minute(s) and %d checks: %vms", duration, len(snapshots), avg)

		err := db.WriteSeries(series)
		if err != nil {
			panic(err)
		}
	}
}

func runTCP(host string, port int, resChan chan *Snapshot) {
	snap := &Snapshot{}

	snap.Host = host
	snap.Port = port
	snap.Proto = "TCP"

	snap.Start = time.Now()
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))

	if err != nil {
		snap.Status = 0
		snap.Duration = float64(0)
		snap.LocalIP, snap.LocalPort = h.ParseNetAddress(c.LocalAddr().String())
		snap.End = time.Now()

		resChan <- snap
		return
	}

	_, err = c.Write(make([]byte, 8))

	if err != nil {
		snap.LocalIP, snap.LocalPort = h.ParseNetAddress(c.LocalAddr().String())
		snap.End = time.Now()

		resChan <- snap
		return
	}

	snap.End = time.Now()
	dur := snap.End.Sub(snap.Start)

	snap.Duration = dur.Seconds() * 1e3
	snap.LocalIP, snap.LocalPort = h.ParseNetAddress(c.LocalAddr().String())
	snap.Status = 1

	if err := c.Close(); err != nil {
		snap.Status = 0
		resChan <- snap
		return
	}

	resChan <- snap
	return
}
