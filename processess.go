package main

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func getPID(proc_name string) int {
	const proc_dir string = "/proc"
	if os.Chdir(proc_dir) != nil {
		return -1
	}

	files, err := ioutil.ReadDir(".")
	if err != nil {
		return -1
	}

	for _, file := range files {
		// Ignore files, we only care about directories
		if file.IsDir() {
			// Our directory name should convert to integer
			// if it's a PID
			pid, err := strconv.Atoi(file.Name())
			if err != nil {
				continue
			}

			/*
				From https://github.com/brgl/busybox/blob/master/libbb/find_pid_by_name.c:

					In Linux we have three ways to determine "process name":
					1. /proc/PID/stat has "...(name)...", among other things. It's so-called "comm" field.
					2. /proc/PID/cmdline's first NUL-terminated string. It's argv[0] from exec syscall.
					3. /proc/PID/exe symlink. Points to the running executable file.
					kernel threads:
						comm: thread name
						cmdline: empty
						exe: <readlink fails>
					executable
						comm: first 15 chars of base name
						(if executable is a symlink, then first 15 chars of symlink name are used)
						cmdline: argv[0] from exec syscall
						exe: points to executable (resolves symlink, unlike comm)
					script (an executable with #!/path/to/interpreter):
						comm: first 15 chars of script's base name (symlinks are not resolved)
						cmdline: /path/to/interpreter (symlinks are not resolved)
						(script name is in argv[1], args are pushed into argv[2] etc)
						exe: points to interpreter's executable (symlinks are resolved)
						If FEATURE_PREFER_APPLETS=y (and more so if FEATURE_SH_STANDALONE=y),
						some commands started from busybox shell, xargs or find are started by
						execXXX("/proc/self/exe", applet_name, params....)
						and therefore comm field contains "exe".

				Therefore we parse the resolved exe symlink, this should cover most of our needs...

			*/
			// script
			comm, err := os.Open(file.Name() + "/comm")
			if err != nil {
				return -1
			}
			defer comm.Close()
			scanner := bufio.NewScanner(comm)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), proc_name) {
					return pid
				}
			}
			if err := scanner.Err(); err != nil {
				return -1
			}
			// executable
			exe, err := os.Readlink(file.Name() + "/exe")
			if err != nil {
				continue
			}
			if strings.Contains(exe, proc_name) {
				log.Println("PID of " + proc_name + " is " + strconv.Itoa(pid))
				return pid
			}
		}
	}
	return -1
}

func runScriptAndCheck(script string, args string, proc string, start bool) bool {
	cmd := exec.Command(script)
	if len(args) > 0 {
		cmd = exec.Command(script, "\""+args+"\"")
	}
	log.Println("Launching ", cmd)
	err := cmd.Run()
	if err != nil {
		return false
	}

	if start {
		log.Print("Check if running ")
		for getPID(proc) == -1 {
			log.Print(".")
			time.Sleep(10 * time.Second)
		}
		log.Println(" OK")
		return true
	} else {
		log.Print("Check if still running ")
		count := 0
		for count < 10 {
			if getPID(proc) != -1 {
				return true
			}
			count++
			log.Print(".")
			time.Sleep(10 * time.Second)
		}
		return false
	}
}
