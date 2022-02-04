package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/hekmon/transmissionrpc/v2"
)

type TransmissionStats struct {
	total       int
	done        int
	downloading int
	seeding     int
	stopped     int
}

func getTorrents(config *Config) (TransmissionStats, error) {
	var status TransmissionStats
	tc, err := transmissionrpc.New(config.RPC.Host, config.RPC.User, config.RPC.Pass, &transmissionrpc.AdvancedConfig{Port: toUint16(config.RPC.Port)})
	if err != nil {
		return status, errors.New("unable to create interface")
	}
	torrents, err := tc.TorrentGetAll(context.TODO())
	if err != nil {
		return status, errors.New("unable to get torrents statistics")
	}

	status.total = len(torrents)
	for _, torrent := range torrents {
		if *torrent.IsFinished {
			status.done++
			continue
		}
		switch *torrent.Status {
		case transmissionrpc.TorrentStatusStopped:
			if *torrent.LeftUntilDone > 0 {
				status.downloading++
			} else {
				status.stopped++
			}
		case transmissionrpc.TorrentStatusDownloadWait:
			status.downloading++
		case transmissionrpc.TorrentStatusDownload:
			status.downloading++
		case transmissionrpc.TorrentStatusSeedWait:
			status.seeding++
		case transmissionrpc.TorrentStatusSeed:
			status.seeding++
		}
	}
	return status, nil
}

func checkTorrentsDownloading(config *Config) bool {
	torrents_status, err := getTorrents(config)
	if err != nil {
		return false
	}
	return torrents_status.downloading > 0
}

func checkTransmissionSocket(config *Config, wanted_ip string) {
	log.Println("Check if Transmission on " + wanted_ip)
	const proc_name string = "transmission-daemon"
	port_open := checkOpenPort(wanted_ip, config.RPC.Port)

	if !port_open {
		log.Println("Transmission not correctly binded")
		log.Println("Stopping Transmission: ")

		tc, err := transmissionrpc.New(config.RPC.Host, config.RPC.User, config.RPC.Pass, &transmissionrpc.AdvancedConfig{Port: toUint16(config.RPC.Port)})
		if err == nil {
			tc.SessionClose(context.TODO())
		}

		for getPID(proc_name) != -1 {
			log.Println("Waiting for daemon to die")
			time.Sleep(10 + time.Second)
		}

		log.Println("Starting Transmission: ")
		args := "/usr/bin/" + proc_name + " --bind-address-ipv4 " + wanted_ip + " -x " + config.Files.Transpid
		for !runProcessAndCheck(config, args, proc_name, true) {
			log.Println("Error launching Transmission")
			time.Sleep(10 + time.Second)
		}
		log.Println("Transmission launched")
		log.Println("Waiting Transmission RPC interface ")
		for !checkOpenPort(config.RPC.Host, config.RPC.Port) {
			time.Sleep(10 + time.Second)
		}
		log.Println("Transmission ready")
	} else {
		log.Println("Correct Transmission binding")
	}
}

type ids []int64

func (s ids) contains(e int64) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func checkSeedNeed(config *Config) bool {
	tc, err := transmissionrpc.New(config.RPC.Host, config.RPC.User, config.RPC.Pass, &transmissionrpc.AdvancedConfig{Port: toUint16(config.RPC.Port)})
	if err != nil {
		return false
	}

	torrents, err := tc.TorrentGetAll(context.TODO())
	if err != nil {
		return false
	}

	var id_to_start ids
	for _, torrent := range torrents {
		doneDate := *torrent.DoneDate
		if torrent.DoneDate == nil {
			id_to_start = append(id_to_start, *torrent.ID)
		}
		if *torrent.UploadRatio < 1 {
			newtorrent := (time.Since(doneDate).Hours() / 24) < 10
			if *torrent.IsPrivate && newtorrent {
				if !id_to_start.contains(*torrent.ID) {
					id_to_start = append(id_to_start, *torrent.ID)
				}
			}
			for _, tracker := range torrent.Trackers {
				for _, trackerid := range config.PrivateTrackers {
					if newtorrent && (strings.Contains(tracker.Announce, trackerid) || strings.Contains(tracker.Scrape, trackerid)) {
						if !id_to_start.contains(*torrent.ID) {
							id_to_start = append(id_to_start, *torrent.ID)
						}
					}
				}
			}
		}
	}
	if len(id_to_start) > 0 {
		tc.TorrentStartNowIDs(context.TODO(), id_to_start)
		return true
	}
	return false
}
