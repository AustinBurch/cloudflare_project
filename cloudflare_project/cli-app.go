package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// IPAddress is the address we will be targeting
type IPAddress struct {
	address net.IP

	hostname string

	ttl int
}

// In the process of understanding how to implement IPV6 support and
// reporting TTL "time exceeded" messages.
func main() {

	var addr IPAddress
	var numPings, packetLoss int
	var totalTime time.Duration

	addr.address, addr.hostname, addr.ttl = GetInput()
	var ipAddr *net.IPAddr
	ipAddr, err := net.ResolveIPAddr("ip4", addr.address.String())
	if err != nil {
		log.Fatal(err)
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		avg := time.Duration(int64(totalTime) / int64(numPings))

		fmt.Printf("\nTotal Pings to %v: %v; Average Time: %v; Packet Loss: %v\n",
			ipAddr, numPings, avg, packetLoss)
		os.Exit(0)
	}()

	for {
		targetAddr, msg, duration, ttl, err := Ping(ipAddr, addr.ttl)
		if err != nil {
			log.Fatal(err)
			packetLoss++
		}
		fmt.Printf("Reply From: %v, Message: %v, Duration: %v, TTL: %v\n",
			targetAddr, msg, duration, ttl)
		numPings++

		totalTime += duration
		time.Sleep(3 * time.Second)
	}

}

// GetInput gets the hostname or IP address to send requests to
func GetInput() (addr net.IP, hostname string, ttl int) {
	fmt.Println("Enter a hostname or IP address:")
	var input string
	//	fmt.Scanln(&input)
	input = "8.8.4.4"
	fmt.Println("Enter TTL: ")

	fmt.Scanln(&ttl)

	addr = net.ParseIP(input)
	if addr == nil {
		hostname := input
		return addr, hostname, ttl
	}

	return addr, hostname, ttl
}

// Ping pings the target ipaddress
func Ping(ipAddr *net.IPAddr, ttl int) (*net.IPAddr, []byte, time.Duration,
	int, error) {

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	conn.IPv4PacketConn().SetTTL(ttl)
	if err != nil {
		log.Fatal(err)
	}
	message := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte(""),
		},
	}

	defer conn.Close()

	b, err := message.Marshal(nil)

	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	n, err := conn.WriteTo(b, ipAddr)

	if err != nil {
		log.Fatal(err)
	} else if n != len(b) {
		return ipAddr, b, time.Since(start), ttl,
			fmt.Errorf("%v Packet Lost", ipAddr)
	}

	reply := make([]byte, 1500)
	err = conn.SetDeadline(time.Now().Add(10 * time.Second))
	if err != nil {
		log.Fatal(err)
	}
	n, target, err := conn.ReadFrom(reply)
	if err != nil {
		log.Fatal(err)
	}
	duration := time.Since(start)

	response, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		log.Fatal(err)
	}

	ttl, err = conn.IPv4PacketConn().TTL()

	if err != nil {
		log.Fatal(err)
	}
	switch response.Type {
	case ipv4.ICMPTypeEchoReply:
		return ipAddr, b, duration, ttl, nil
	default:
		return ipAddr, b, 0, ttl, fmt.Errorf("Packet loss: %v From: %v", response, target)
	}
}
