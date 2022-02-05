package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"
)

type RPCInfo struct {
	Host   string `json:"host"`
	Port   string `json:"port"`
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Socket string `json:"rpc"`
}

type ScheduleTime struct {
	Hour int `json:"hour"`
	Min  int `json:"min"`
}

type Schedule struct {
	Start ScheduleTime `json:"start"`
	Stop  ScheduleTime `json:"stop"`
}

type Files struct {
	Start string `json:"start"`
	Stop  string `json:"stop"`
	Pid   string `json:"pid"`
	Bind  string `json:"bind"`
}

type Commands struct {
	StartTrans string `json:"trans_start"`
	StartVPN   string `json:"vpn_start"`
	StopVPN    string `json:"vpn_stop"`
}

type trackers []string

type Config struct {
	Eth0     string   `json:"eth0_ip"`
	Drives   []string `json:"drives"`
	Files    Files    `json:"files"`
	RPC      RPCInfo  `json:"rpc"`
	Schedule struct {
		Week    Schedule `json:"week"`
		Weekend Schedule `json:"weekend"`
	} `json:"schedule"`
	Commands        Commands `json:"commands"`
	PrivateTrackers trackers `json:"private_trackers"`
}

func toUint16(n string) uint16 {
	uintport, _ := strconv.ParseUint(n, 10, 16)
	return uint16(uintport)
}

func loadConfig(configPath string) (Config, error) {
	var config Config
	if configPath == "" {
		return config, errors.New("Config path not provided")
	}

	log.Println("Running.")

	configFile, err := os.Open(configPath)
	if err != nil {
		return config, fmt.Errorf("error with opening config file %s: %v", configPath, err)
	}
	defer configFile.Close()

	configData, err := ioutil.ReadAll(configFile)
	if err != nil {
		return config, fmt.Errorf("error with reading config file %s: %v", configPath, err)
	}

	err = json.Unmarshal(configData, &config)
	if err != nil {
		return config, fmt.Errorf("error parsing config file %s: %v", configPath, err)
	}
	return config, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func checkHDD(conf *Config) bool {
	for _, d := range conf.Drives {
		if !exists(d) {
			return false
		}
	}
	return true
}

func checkTime(config *Config) bool {
	t := time.Now()
	schedule := config.Schedule.Week
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		schedule = config.Schedule.Weekend
	}
	start := time.Date(t.Year(), t.Month(), t.Day(), schedule.Start.Hour, schedule.Start.Min, 0, 0, t.Location())
	stop := time.Date(t.Year(), t.Month(), t.Day(), schedule.Stop.Hour, schedule.Stop.Min, 0, 0, t.Location())

	return t.After(start) && t.Before(stop)
}

type SystemState int

const (
	force_offline SystemState = iota
	force_online
	req_offline
	req_online
)

func getSystemState(config *Config) SystemState {
	if exists(config.Files.Stop) {
		return force_offline
	}
	if exists(config.Files.Start) {
		return force_online
	}
	time_is_right := checkTime(config)
	data_to_transfer := checkTorrentsDownloading(config) || checkSeedNeed(config)
	if !time_is_right || !data_to_transfer {
		return req_offline
	}
	if time_is_right && data_to_transfer {
		return req_online
	}
	return req_offline
}

func main() {
	// Load configuration

	configPath := flag.String("config", "./transmgr.json", "Configuration path")

	flag.Parse()
	config, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalln("Failed configuration loading: ", err)
	}

	if !exists(config.Files.Pid) {
		os.OpenFile(config.Files.Pid, os.O_RDONLY|os.O_CREATE, 0666)
		local_ip_address := localhost

		if !checkHDD(&config) {
			log.Println("NO HARD DRIVES!!!")
			log.Println("Stopping Transmission: ")
			//command = "/usr/bin/transmission-remote %s:%s -n %s:%s --exit" % (rpc_param['address'], rpc_param['port'], rpc_param['user'], rpc_param['password'])
			//if run_process_and_check(command.split(' '), "transmission-daemon", False):
			//    log.Println("Transmission stopped")
			//else:
			//    log.Println("Error stopping Transmission")
			//    commandkill = "sudo killall transmission-daemon"
			//    if run_process_and_check(commandkill.split(' '), "transmission-daemon", False):
			//        log.Println("Transmission killed")
			//    else:
			//        log.Println("Error killing Transmission")
		} else {
			port_open := checkOpenPort(local_ip_address, config.RPC.Socket)

			if !port_open {
				if err != nil {
					log.Fatalln("Failed creating client: ", err)
				}
				switch getSystemState(&config) {
				case force_offline:
					log.Println("Transmission forced OFFLINE")
					local_ip_address = manageVPN(&config, false)
				case force_online:
					log.Println("Transmission forced ONLINE")
					local_ip_address = manageVPN(&config, true)
				case req_offline:
					log.Println("Transmission should be OFFLINE")
					local_ip_address = manageVPN(&config, false)
				case req_online:
					log.Println("Transmission should be ONLINE")
					local_ip_address = manageVPN(&config, true)
				}
			}
			checkTransmissionSocket(&config, local_ip_address)
		}

		log.Println("Done.")
		os.Remove(config.Files.Pid)
	}
}
