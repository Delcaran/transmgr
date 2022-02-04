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

	"github.com/hekmon/transmissionrpc/v2"
)

type RPCInfo struct {
	Host   string `json:"host"`
	Port   string `json:"port"`
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Socket string
}

type ScheduleTime struct {
	hour int
	min  int
}

type Schedule struct {
	start ScheduleTime
	stop  ScheduleTime
}

type Files struct {
	start    string
	stop     string
	pid      string
	transpid string
}

type Commands struct {
	vpn_start string
	vpn_stop  string
}

type trackers []string

type Config struct {
	eth0_ip  string
	drives   []string
	files    Files
	RPC      RPCInfo `json:"rpc"`
	schedule struct {
		week    Schedule
		weekend Schedule
	}
	commands         Commands
	private_trackers trackers
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
	for _, d := range conf.drives {
		if !exists(d) {
			return false
		}
	}
	return true
}

func checkTime(config *Config) bool {
	t := time.Now()
	schedule := config.schedule.week
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		schedule = config.schedule.weekend
	}
	start := time.Date(t.Year(), t.Month(), t.Day(), schedule.start.hour, schedule.start.min, 0, 0, t.Location())
	stop := time.Date(t.Year(), t.Month(), t.Day(), schedule.stop.hour, schedule.stop.min, 0, 0, t.Location())

	return t.After(start) && t.Before(stop)
}

type SystemState int

const (
	force_offline SystemState = iota
	force_online
	req_offline
	req_online
)

func getSystemState(config *Config, tc *transmissionrpc.Client) SystemState {
	if exists(config.files.stop) {
		return force_offline
	}
	if exists(config.files.start) {
		return force_online
	}
	time_is_right := checkTime(config)
	data_to_transfer := checkTorrentsDownloading(tc) || checkSeedNeed(config, tc)
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

	if !exists(config.files.pid) {
		os.OpenFile(config.files.pid, os.O_RDONLY|os.O_CREATE, 0666)
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
			uintport, _ := strconv.ParseUint(config.RPC.Port, 10, 16)
			tclient, err := transmissionrpc.New(config.RPC.Host, config.RPC.User, config.RPC.Pass, &transmissionrpc.AdvancedConfig{Port: uint16(uintport)})
			if err != nil {
				log.Fatalln("Failed creating client: ", err)
			}
			switch getSystemState(&config, tclient) {
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
			checkTransmissionSocket(&config, tclient, local_ip_address)
		}

		log.Println("Done.")
		os.Remove(config.files.pid)
	}
}
