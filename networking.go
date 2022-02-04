package transmgr

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

func get_local_online_ip() string {
	conn, err := net.Dial("udp", testhost)
	if err != nil {
		return localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func start_vpn(config *Config) string {
	log.Println("Starting VPN: ")
	if run_process_and_check(config.commands.vpn_start, nil, "openvpn", true) {
		log.Println("VPN running")
	} else {
		log.Println("Error launching VPN")
		return localhost
	}
	log.Println("Waiting VPN connection")
	ip := get_local_online_ip()
	for ip == config.eth0_ip {
		log.Println("Still on " + config.eth0_ip)
		time.Sleep(10 * time.Second)
	}
	log.Println("Now on " + ip)
	return ip
}

func stop_vpn(config *Config) string {
	log.Print("Stopping VPN: ")
	if run_process_and_check(config.commands.vpn_stop, nil, "openvpn", false) {
		log.Println("OK")
	} else {
		log.Println("Error stopping VPN")
		return localhost
	}
	log.Println("Waiting VPN shutdown")
	ip := get_local_online_ip()
	for ip != config.eth0_ip {
		log.Println("Still on " + ip)
		time.Sleep(10 * time.Second)
	}
	log.Println("Now on " + ip)
	return localhost
}

func restart_vpn(config *Config) string {
	stop_vpn(config)
	return start_vpn(config)
}

func check_vpn_connection() bool {
	pid := pidof("openvpn")
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

func manage_vpn(config *Config, wanted_online bool) string {
	local_ip_address := get_local_online_ip()
	VPN_online := local_ip_address != config.eth0_ip
	if wanted_online {
		if VPN_online {
			if check_vpn_connection() {
				log.Println("VPN ONLINE")
			} else {
				log.Println("VPN needs restart")
				restart_vpn(config)
			}
			return local_ip_address
		} else {
			return start_vpn(config)
		}
	} else {
		if VPN_online {
			return stop_vpn(config)
		} else {
			log.Println("VPN OFFLINE")
			return localhost
		}
	}
}
