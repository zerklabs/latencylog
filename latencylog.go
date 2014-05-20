package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"
)

type Snapshot struct {
	Start         time.Time
	End           time.Time
	Duration      time.Duration
	Host          string
	Port          int
	LocalAddress  string
	RemoteAddress string
}

var (
	host      = flag.String("host", "", "Target IP to connect to. Required")
	port      = flag.Int("port", 0, "Target port to connect to. Required")
	duration  = flag.Int("duration", 1, "How long to run for (minutes)")
	snapshots []*Snapshot
)

func main() {
	flag.Parse()

	if len(*host) == 0 {
		fmt.Println("Host required")

		flag.PrintDefaults()
		return
	}

	if *port == 0 {
		fmt.Println("Port required")

		flag.PrintDefaults()
		return
	}

	snapshots = make([]*Snapshot, 0)

	tickChan := time.NewTicker(time.Second * 1)

	go func() {
		for _ = range tickChan.C {
			res, err := run(*host, *port)

			if err != nil {
				log.Fatal(err)
			}

			snapshots = append(snapshots, res)

			log.Printf("%v, %s <--> %s", res.Duration, res.LocalAddress, res.RemoteAddress)
		}
	}()

	sleepFor, _ := time.ParseDuration(fmt.Sprintf("%dm", *duration))
	log.Printf("Running for: %s", sleepFor)
	time.Sleep(sleepFor)
	tickChan.Stop()

	if len(snapshots) > 0 {
		total := float64(0)
		log.Printf("Snapshots: %d", len(snapshots))

		for _, v := range snapshots {
			total += v.Duration.Seconds() * 1e3
		}

		avg := total / float64(len(snapshots))
		log.Printf("Average after %d minute(s) and %d checks: %vms", *duration, len(snapshots), avg)
	}
}

func run(host string, port int) (*Snapshot, error) {
	start := time.Now()
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", host, port))

	if err != nil {
		return &Snapshot{}, err
	}

	c.Write(make([]byte, 8))

	end := time.Now()
	dur := end.Sub(start)

	snap := &Snapshot{
		Start:         start,
		End:           end,
		Duration:      dur,
		Host:          host,
		Port:          port,
		LocalAddress:  c.LocalAddr().String(),
		RemoteAddress: c.RemoteAddr().String(),
	}

	if err := c.Close(); err != nil {
		return snap, err
	}

	return snap, nil
}
