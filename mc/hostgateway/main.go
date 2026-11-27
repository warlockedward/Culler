// Command hostgateway is the A31 host-gateway reachability probe.
//
// It proves the connect half of the Linux host-gateway capability the
// resident injects (`--add-host host.docker.internal:host-gateway`): one
// process listens on the host side of the bridge, one runs inside a
// container that carries the production flag and dials the mapped name.
// The tool never spawns programs and never touches the spine; the listen
// side binds the bridge gateway address, so nothing is exposed beyond the
// docker bridge the container already routes through.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

var (
	mode = flag.String("mode", "dial", "listen | dial")
	addr = flag.String("addr", "host.docker.internal:17999", "address to dial (mode=dial) or listen on (mode=listen)")
)

func main() {
	flag.Parse()
	switch *mode {
	case "listen":
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hostgateway listen: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("listening on %s\n", ln.Addr().String())
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serve(conn)
		}
	case "dial":
		start := time.Now()
		conn, err := net.DialTimeout("tcp", *addr, 10*time.Second)
		if err != nil {
			fmt.Printf("dial=FAIL target=%s err=%v\n", *addr, err)
			os.Exit(1)
		}
		defer conn.Close()
		if _, err := conn.Write([]byte("ping\n")); err != nil {
			fmt.Printf("dial=WRITE-FAIL err=%v\n", err)
			os.Exit(1)
		}
		line, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil || line != "pong\n" {
			fmt.Printf("dial=ECHO-FAIL got=%q err=%v\n", line, err)
			os.Exit(1)
		}
		fmt.Printf("dial=OK target=%s rtt_ms=%.2f echo=pong\n", *addr,
			float64(time.Since(start).Microseconds())/1000.0)
	default:
		fmt.Fprintln(os.Stderr, "hostgateway: unknown mode")
		os.Exit(2)
	}
}

func serve(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		if scanner.Text() == "ping" {
			if _, err := conn.Write([]byte("pong\n")); err != nil {
				return
			}
		}
	}
}
