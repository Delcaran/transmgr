package main

import (
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

func getLocalOnlineIP() string {
	conn, err := net.Dial("udp", testhost)
	if err != nil {
		return localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func startVPN(config *Config) string {
	log.Println("Starting VPN: ")
	cmd := config.Commands.StartVPN
	if runProcessAndCheck(config, cmd, "openvpn", true) {
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
	cmd := config.Commands.StopVPN
	if runProcessAndCheck(config, cmd, "openvpn", false) {
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
