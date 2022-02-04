package main

import (
	"context"
	"errors"
	"log"
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

func getTorrents(tc *transmissionrpc.Client) (TransmissionStats, error) {
	var status TransmissionStats
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

func checkTorrentsDownloading(tc *transmissionrpc.Client) bool {
	torrents_status, err := getTorrents(tc)
	if err != nil {
		return false
	}
	return torrents_status.downloading > 0
}

func checkTransmissionSocket(config *Config, tc *transmissionrpc.Client, wanted_ip string) {
	log.Println("Check if Transmission on " + wanted_ip)
	const proc_name string = "transmission-daemon"
	port_open := checkOpenPort(wanted_ip, config.RPC.Socket)

	if !port_open {
		log.Println("Transmission not correctly binded")
		log.Println("Stopping Transmission: ")
		tc.SessionClose(context.TODO())

		for getPID(proc_name) != -1 {
			log.Println("Waiting for daemon to die")
			time.Sleep(10 + time.Second)
		}

		log.Println("Starting Transmission: ")
		proc := "transmission-daemon"
		args := []string{"--bind-address-ipv4", wanted_ip, "-x", config.files.transpid}
		for !runProcessAndCheck(proc, args, proc, true) {
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

func (s trackers) contains(e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func checkSeedNeed(config *Config, tc *transmissionrpc.Client) bool {
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
				if newtorrent && (config.private_trackers.contains(tracker.Announce) || config.private_trackers.contains(tracker.Scrape)) {
					if !id_to_start.contains(*torrent.ID) {
						id_to_start = append(id_to_start, *torrent.ID)
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
