package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

/* Discover discovers printers on the network. It returns a slice of
 * pointers to Printer objects. If no printers are found, it returns
 * an empty slice. If an error occurs, it returns nil.
 */
func Discover(timeout time.Duration) ([]*Printer, error) {
	// Create a slice to hold the printers
	printers := []*Printer{}

	addrs, err := getBroadcastAddresses()
	if err != nil {
		return printers, err
	}

	for _, addr := range addrs {

		// Create a new UDP broadcast address
		broadcastAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", addr, 20054))
		if err != nil {
			return nil, err
		}

		// Create a new UDP connection
		conn, err := net.ListenUDP("udp4", nil)
		if err != nil {
			return nil, err
		}
		defer conn.Close()

		if Debug {
			log.Printf("-- Discovering on %s", broadcastAddr)
		}

		// Set a timeout for the connection
		conn.SetDeadline(time.Now().Add(timeout))

		// Send the discover message
		_, err = conn.WriteTo([]byte("discover"), broadcastAddr)
		if err != nil {
			return nil, err
		}

		// Create a buffer to hold the response
		buf := make([]byte, 1500)

		// Loop until the timeout is reached
		for {
			// Read the response
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				// If the error is a timeout, break out of the loop
				if err, ok := err.(net.Error); ok && err.Timeout() {
					break
				}
				return nil, err
			}

			if Debug {
				log.Printf("-- Discover got %d bytes %s", n, buf[:n])
			}

			// Parse the response into a Printer object
			printer, err := NewPrinter(buf[:n])
			if err != nil {
				continue
			}

			// Add the printer to the slice
			printers = append(printers, printer)
		}
	}

	// Return the slice of printers
	return printers, nil
}

func getBroadcastAddresses() ([]string, error) {
	ifs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	addrs := make([]string, 0)
	for _, address := range ifs {
		var ip net.IP
		switch typedAddress := address.(type) {
		case *net.TCPAddr:
			ip = typedAddress.IP.To4()
		case *net.UDPAddr:
			ip = typedAddress.IP.To4()
		case *net.IPNet:
			ip = typedAddress.IP.To4()
		default:
		}

		if ip != nil && len(ip) == net.IPv4len && ip.IsGlobalUnicast() {
			ip[3] = 255
			addrs = append(addrs, ip.String())
		}
	}
	return addrs, nil
}
