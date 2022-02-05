package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

const localhost string = "127.0.0.1"
const testhost string = "8.8.8.8:80"

func checkOpenPort(host string, port string) bool {
	timeout := time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
	if err != nil {
		return false
	}
	if conn != nil {
		defer conn.Close()
		return true
	}
	return false
}

func GetInterfaceIpv4Addr(interfaceName string) (addr string, err error) {
	var (
		ief      *net.Interface
		addrs    []net.Addr
		ipv4Addr net.IP
	)
	if ief, err = net.InterfaceByName(interfaceName); err != nil { // get interface
		return
	}
	if addrs, err = ief.Addrs(); err != nil { // get addresses
		return
	}
	for _, addr := range addrs { // get ipv4 address
		if ipv4Addr = addr.(*net.IPNet).IP.To4(); ipv4Addr != nil {
			break
		}
	}
	if ipv4Addr == nil {
		return "", fmt.Errorf("interface %s don't have an ipv4 address", interfaceName)
	}
	return ipv4Addr.String(), nil
}

func getLocalOnlineIP() string {
	for _, i := range []string{"tun0", "eth0", "lo"} {
		addr, err := GetInterfaceIpv4Addr(i)
		if err == nil {
			return addr
		}
	}
	return localhost
}

func startVPN(config *Config) string {
	log.Println("Starting VPN: ")
	if runScriptAndCheck(config.Commands.StartVPN, "", "openvpn", true) {
		log.Println("VPN running")
	} else {
		log.Println("Error launching VPN")
		return localhost
	}
	log.Println("Waiting VPN connection")
	ip := getLocalOnlineIP()
	for ip == config.Eth0 {
		log.Println("Still on " + config.Eth0)
		time.Sleep(10 * time.Second)
	}
	log.Println("Now on " + ip)
	return ip
}

func stopVPN(config *Config) string {
	log.Print("Stopping VPN: ")
	if runScriptAndCheck(config.Commands.StopVPN, "", "openvpn", false) {
		log.Println("OK")
	} else {
		log.Println("Error stopping VPN")
		return localhost
	}
	log.Println("Waiting VPN shutdown")
	ip := getLocalOnlineIP()
	for ip != config.Eth0 {
		log.Println("Still on " + ip)
		time.Sleep(10 * time.Second)
	}
	log.Println("Now on " + ip)
	return localhost
}

func restartVPN(config *Config) string {
	stopVPN(config)
	return startVPN(config)
}

func checkConnectionVPN() bool {
	pid := getPID("openvpn")
	if pid == -1 {
		return false
	}
	conn, err := net.Dial("udp", testhost)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

func manageVPN(config *Config, wanted_online bool) string {
	local_ip_address := getLocalOnlineIP()
	VPN_online := local_ip_address != config.Eth0
	if wanted_online {
		if VPN_online {
			if checkConnectionVPN() {
				log.Println("VPN ONLINE")
			} else {
				log.Println("VPN needs restart")
				restartVPN(config)
			}
			return local_ip_address
		} else {
			return startVPN(config)
		}
	} else {
		if VPN_online {
			return stopVPN(config)
		} else {
			log.Println("VPN OFFLINE")
			return localhost
		}
	}
}
