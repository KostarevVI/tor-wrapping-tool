package main

import (
	. "./constants"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func check(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

// execSh created for convenient command execution. Returns stdout and err+stderr
func execSh(command string) (string, string) {
	cmd := exec.Command("sh", "-c", command)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	errStr := ""
	if err != nil {
		errStr = fmt.Sprintln(fmt.Sprint(err) + ": " + stderr.String())
	}
	return stdout.String(), errStr
}

// appendTextIfAbsent created for config editing simplification
func addTextIfAbsent(path string, text string, override bool) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	check(err)

	buff := make([]byte, 10)
	var fileByte []byte
	for {
		bytesRead, _ := f.Read(buff)
		if bytesRead == 0 {
			break
		}
		fileByte = append(fileByte, buff...)
	}

	if !strings.Contains(string(fileByte), text) {
		if override {
			_, err = f.Seek(0, io.SeekStart)
			check(err)

			err = f.Truncate(0)
			check(err)

			_, err = fmt.Fprintln(f, text)
			check(err)

		} else {
			_, err = fmt.Fprintln(f, text)
			check(err)

			err = f.Close()
			check(err)
		}
		fmt.Printf("Appending new data to %s file\n", path)
	} else {
		fmt.Printf("File %s is ok\n", path)
	}
}

// printHelp prints help to stdout
func printHelp() {
	fmt.Println(`start		Run Torwrapper for this system 
stop		Stop Torwrapper and restore settings
restart		Consequent launch of "stop" and "start"
status		Check if Torwrapper is available (on/off state)
changeid	TOR restart for identity (IP) change
myip		Learn IP address
dns			Change present DNS for OpenNIC DNS 
updbridges	Update TOR bridges from the GitHub repo`)
}

// start launches torwrapper service
func start() {
	if !isActive() {
		// Setting-up configs and back-ups
		// Updating TORRC
		execSh(BACKUP_TORRC_CMD)
		addTextIfAbsent("/etc/tor/torrc", TORRC_CONFIG, true)

		// Updating resolv.conv for DNS
		execSh(BACKUP_RESOLV_CONV_CMD)
		addTextIfAbsent("/etc/resolv.conf", RESOLV_CONV_CONFIG, false)
		execSh("sudo /etc/init.d/tor restart")

		// Update firewall
		execSh(BACKUP_IPTABLES_RULES_CMD)
		execSh(CLEAR_IPTABLES_RULES)
		execSh(APPLY_TORWRAPPER_IPTABLES_RULES)

		// Launch service for status tracking
		execSh("sudo systemctl start torwrapper.service")

		fmt.Println("Torwrapper service has been started. Connecting to TOR Network " +
			"- if it fails, bridges would be automatically applied.\nEstimate waiting time - 30 seconds...")
		time.Sleep(15 * time.Second)

		timeout := time.After(15 * time.Second)
		done := make(chan bool)
		var isConnected bool

		go func() {
			for {
				select {
				case <-timeout:
					fmt.Println("Connection timeout achieved (30 seconds)\n" +
						"\nTrying to connect with TOR bridges\nEstimate waiting time - 30 seconds...")
					done <- false
				default:
					if stdout, _ := execSh(CHECK_TOR_CONNECTION_CMD); stdout != "" {
						fmt.Printf("\nConnected successfully with TOR IP address %s\n", stdout)
						done <- true
						return
					}
				}
				time.Sleep(1 * time.Second)
			}
		}()

		isConnected = <-done
		if isConnected {
			return
		}

		content, err := ioutil.ReadFile("/etc/tor/bridges.txt")
		check(err)
		addTextIfAbsent("/etc/tor/torrc", string(content), false)
		execSh("sudo /etc/init.d/tor restart")
		time.Sleep(15 * time.Second)

		timeout = time.After(15 * time.Second)
		done = make(chan bool)

		go func() {
			for {
				select {
				case <-timeout:
					fmt.Println("\n\nConnection timeout achieved (5 minutes)\n" +
						"Try to update bridges with \"torwrapper updbridges\" or " +
						"add them manually in /etc/tor/bridges.txt from https://t.me/GetBridgesBot\n" +
						"Program is terminated. Please, start Torwrapper again")
					done <- false
					return
				default:
					if stdout, _ := execSh(CHECK_TOR_CONNECTION_CMD); stdout != "" {
						fmt.Printf("Connected successfully with TOR IP address %s\n", stdout)
						done <- true
						return
					}
				}
				time.Sleep(1 * time.Second)
			}
		}()

		isConnected = <-done
		if !isConnected {
			// stop the service if failed to connect to the TOR network
			stop()
		}
	} else {
		fmt.Println("Cannot start already working service")
	}
}

// stop kills torwrapper service and restore configs
func stop() {
	if isActive() {
		execSh(CLEAR_IPTABLES_RULES)
		execSh(RESTORE_IPTABLE_RULES_CMD)

		execSh(RESTORE_RESOLV_CONV_CMD)

		execSh(RESTORE_TORRC_CMD)

		execSh("sudo systemctl stop torwrapper.service")

		fmt.Println("Torwrapper service has been stopped")
	} else {
		fmt.Println("Service is already offline")
	}
}

// isActive checks if torwrapper service is alive and achievable
func isActive() bool {
	if stdout, _ := execSh(`systemctl -a | grep -F 'torwrapper'`); stdout != "" {
		fmt.Println("Torwrapper status: active")
		return true
	}
	fmt.Println("Torwrapper status: inactive")
	return false
}

// restartTorService changes IP in TOR
func restartTorService() {
	// TODO find how to send signals to TOR service
}

// checkMyIp returns IP address via get request to ident.me
func checkMyIp() {
	resp, err := http.Get("https://ident.me")
	check(err)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		check(err)
	}(resp.Body)
	myIp, err := ioutil.ReadAll(resp.Body)
	check(err)

	fmt.Printf("Your IP address: %s (according to ident.me)\n", myIp)
}

// changeDNS from default system DNS to OpenNIC
func changeDNS() {
	if isActive() {
		resp, err := http.Get("https://api.opennicproject.org/geoip/?resolv")
		check(err)
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			check(err)
		}(resp.Body)

		bodyBytes, err := ioutil.ReadAll(resp.Body)
		check(err)

		addTextIfAbsent("/etc/resolv.conf", string(bodyBytes), false)

		// print the difference (before/after)
		fmt.Printf("DNS has been changed to OpenNIC until reboot")
	} else {
		fmt.Println("To change DNS Torwrapper should be enabled. Run \"torwrapper start\" to do so.")
	}
}

// updateBridges downloads recent private bridges from project's GitHub repo
func updateBridges() {
	execSh("sudo rm /etc/tor/bridges.txt")
	_, stderr := execSh(DOWNLOAD_BRIDGES_CMD)

	if stderr == "" {
		fmt.Println("Bridges have been updated successfully")
	} else {
		fmt.Printf("Some problems occurred:\n%s", stderr)
	}
}

// service is used as tool's availability flag
func service() {
	for {
		fmt.Printf("Doin' useful job...%d", rand.Int())
		time.Sleep(1 * time.Second)
	}
}

func main() {
	args := os.Args[1:]

	if len(args) > 1 || len(args) == 0 {
		fmt.Println("Cannot recognise multiple or zero arguments. Type \"torwrapper help\" for hints.")
		return
	}

	switch args[0] {
	case "help":
		printHelp()
	case "start":
		start()
	case "stop":
		stop()
	case "restart":
		stop()
		start()
	case "status":
		isActive()
	//case "changeid":
	//	restartTorService()
	//	checkMyIp()
	case "myip":
		checkMyIp()
	case "dns":
		changeDNS()
	case "updbridges":
		updateBridges()
	case "service":
		service()
	default:
		fmt.Printf("Unrecognisable argument: \"%s\". Type \"torwrapper help\" for hints.\n", args[0])
	}
	return
}
