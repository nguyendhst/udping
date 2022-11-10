package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	E_Timeout     = "timeout"
	E_ConnRefused = "connection refused (no response)"
)

// run is the struct that is sent to the agent for each module run
type (
	run struct {
		Parameters params
		Results    []result
	}

	// parameters is the struct that is sent to the agent for each module run
	params struct {
		Destination     string `json:"destination"`               // ipv4, ipv6 or fqdn.
		DestinationPort int    `json:"destinationport,omitempty"` // 16 bits integer. Throws an error when used with icmp. Defaults to 80 otherwise.
		Protocol        string `json:"protocol"`                  // icmp, tcp, udp
		Count           int    `json:"count,omitempty"`           // Number of tests
		Timeout         int    `json:"timeout,omitempty"`         // Timeout for individual test. defaults to 5s.
		ipDest          string
	}

	// result is the struct that is returned to the scheduler with the results of a module run
	result struct {
		Success         bool    `json:"success"`                   // Success is true if the module was able to connect to the destination
		Error           string  `json:"error,omitempty"`           // Error contains any error that occurred during the module run
		Destination     string  `json:"destination"`               // Destination is the IP address or hostname of the destination
		DestinationPort float64 `json:"destinationport,omitempty"` // DestinationPort is the port number of the destination
		Protocol        string  `json:"protocol"`                  // Protocol is the protocol used for the ping
		RTT             float64 `json:"rtt,omitempty"`             // RTT is the round trip time of the packet
	}
)

// ValidateParameters validates the parameters that are sent to the module
func (r *run) ValidateParameters() (err error) {
	// tcp and udp pings must have a destination port
	if r.Parameters.Protocol != "icmp" && (r.Parameters.DestinationPort < 0 || r.Parameters.DestinationPort > 65535) {
		return fmt.Errorf("%s ping requires a valid destination port between 0 and 65535, got %d",
			r.Parameters.Protocol, r.Parameters.DestinationPort)
	}
	// if the destination is a FQDN, resolve it and take the first IP returned as the dest
	ips, err := net.LookupHost(r.Parameters.Destination)
	ip := ""
	// Get ip based on destination.
	// if ip == nil, destination may not be a hostname.
	if err != nil {
		ip = r.Parameters.Destination
	} else {
		if len(ips) == 0 {
			return fmt.Errorf("FQDN does not resolve to any known ip")
		}
		ip = ips[0]
	}

	// check the format of the destination IP
	ip_parsed := net.ParseIP(ip)
	if ip_parsed == nil {
		return fmt.Errorf("destination IP is invalid: %v", ip)
	}
	r.Parameters.ipDest = ip

	// if timeout is not set, default to 5 seconds
	if r.Parameters.Timeout == 0.0 {
		r.Parameters.Timeout = 5.0
	}

	// if count of pings is not set, default to 3
	if r.Parameters.Count == 0.0 {
		r.Parameters.Count = 3
	}
	return
}

// pingUdp sends a UDP packet to a destination ip:port to determine if it is open or closed.
// Because UDP does not reply to connection requests, a lack of response may indicate that the
// port is open, or that the packet got dropped. We chose to be optimistic and treat lack of
// response (connection timeout) as an open port.
func (r *run) pingUdp() error {
	// Make it ip:port format
	destination := r.Parameters.Destination + ":" + fmt.Sprintf("%d", int(r.Parameters.DestinationPort))

	c, err := net.Dial("udp", destination)
	if err != nil {
		log.Println(err)
		return err
	}

	c.Write([]byte("Ping!Ping!Ping!"))
	c.SetReadDeadline(time.Now().Add(time.Duration(r.Parameters.Timeout) * time.Second))
	defer c.Close()

	rb := make([]byte, 1500)

	if _, err := c.Read(rb); err != nil {
		// If connection timed out, we return E_Timeout
		if e := err.(*net.OpError).Timeout(); e {
			return fmt.Errorf(E_Timeout)
		}
		if strings.Contains(err.Error(), "connection refused") {
			return fmt.Errorf(E_ConnRefused)
		}
		return fmt.Errorf("read Error: %v", err.Error())
	} else {
		fmt.Printf("%v bytes from %v", len(rb), destination)
	}
	return nil
}

func (r *run) Run() error {
	err := r.ValidateParameters()
	if err != nil {
		return err
	}

	if r.Parameters.Protocol == "udp" {
		// if the protocol is udp, we use our own ping function
		for i := 0; i < r.Parameters.Count; i++ {
			start := time.Now()
			fmt.Printf("[%v] pinging %s:%d\n", i, r.Parameters.Destination, r.Parameters.DestinationPort)
			err := r.pingUdp()
			if err != nil {
				if err.Error() == E_Timeout {
					r.Results[i].Error = E_Timeout
					r.Results[i].Success = false
				} else if err.Error() == E_ConnRefused {
					r.Results[i].Error = E_ConnRefused
					r.Results[i].Success = true
				} else {
					r.Results[i].Error = err.Error()
					r.Results[i].Success = false
				}

			}
			end := time.Now()
			elapsed := end.Sub(start)
			r.Results[i].RTT = elapsed.Seconds()

			r.Results[i].Destination = r.Parameters.Destination
			r.Results[i].DestinationPort = float64(r.Parameters.DestinationPort)
			r.Results[i].Protocol = r.Parameters.Protocol
		}

	} else {
		return fmt.Errorf("protocol %s is not supported", r.Parameters.Protocol)
	}

	return nil

}
