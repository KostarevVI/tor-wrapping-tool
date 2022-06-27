package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
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
changeid	Tor restart for identity (IP) change
myip		Display IP address
dns		Change present DNS to OpenNIC DNS 
updbridges	Download available NON OBFS4 relays (from the source webpage) that can be used as Tor bridges

To set custom obfs4 bridges - visit https://bridges.torproject.org (with enabled VPN, probably) and copy them to /etc/tor/bridges.txt`)
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

		// Update firewall
		execSh(BACKUP_IPTABLES_RULES_CMD)
		execSh(CLEAR_IPTABLES_RULES)
		execSh(APPLY_TORWRAPPER_IPTABLES_RULES)

		// Launch service for status tracking
		execSh("sudo systemctl start torwrapper.service")

		fmt.Println("Torwrapper service has been started. Connecting to TOR Network " +
			"- if it fails, bridges will be automatically applied\nEstimate waiting time - 20 seconds")

		isConnected := false

		timeout := time.After(20 * time.Second)
		done := make(chan bool)

		execSh("sudo /etc/init.d/tor restart")
		time.Sleep(15 * time.Second)

		go func() {
			for {
				select {
				case <-timeout:
					done <- false
					return
				default:
					go func() {
						if stdout, _ := execSh(CHECK_TOR_IP_CMD); stdout != "" {
							done <- true
							return
						}
					}()
				}
				time.Sleep(1 * time.Second)
			}
		}()

		isConnected = <-done

		if isConnected {
			fmt.Print("\n")
			checkIp()
			return
		}
		fmt.Println("Connection timeout (20 seconds)\n\nTrying to connect with TOR bridges\n" +
			"Estimate waiting time - 1 minute")

		addTextIfAbsent("/etc/tor/torrc", ENABLE_BRIDGES_CONFIG, false)
		content, err := ioutil.ReadFile("/etc/tor/bridges.txt")
		check(err)
		addTextIfAbsent("/etc/tor/torrc", string(content), false)

		counter := 0
		for !isConnected && counter < 3 {
			fmt.Printf("Attempt %d of 3. Trying to connect...\n", counter+1)

			timeout := time.After(20 * time.Second)
			done := make(chan bool)

			execSh("sudo /etc/init.d/tor restart")
			time.Sleep(15 * time.Second)

			go func() {
				for {
					select {
					case <-timeout:
						done <- false
						return
					default:
						go func() {
							if stdout, _ := execSh(CHECK_TOR_IP_CMD); stdout != "" {
								done <- true
								return
							}
						}()
					}
					time.Sleep(1 * time.Second)
				}
			}()

			isConnected = <-done
			counter++
		}

		if isConnected {
			fmt.Print("\n")
			checkIp()
			return
		}

		fmt.Println("\nConnection timeout (1 minute)\n" +
			"Try to update bridges with \"torwrapper updbridges\" or " +
			"add them manually in /etc/tor/bridges.txt from https://bridges.torproject.org\n" +
			"Service will be terminated. Please, start Torwrapper again")

		// stop the service if failed to connect to the TOR network
		stop()
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

// isActive checks if torwrapper service is alive and achievable and returns the bool state
func isActive() bool {
	if stdout, _ := execSh(`systemctl -a | grep -F 'torwrapper'`); stdout != "" {
		fmt.Println("Torwrapper status: active")
		return true
	}
	fmt.Println("Torwrapper status: inactive")
	return false
}

// restartTorService changes IP address without exiting TOR
func restartTorService() {
	if isActive() {
		checkIp()

		torOk := make([]byte, 200)
		torCon, err := net.Dial("tcp", "127.0.0.1:9051")
		check(err)

		_, err = torCon.Write([]byte("authenticate \"\"\n"))
		check(err)
		_, err = torCon.Read(torOk)
		check(err)
		if !strings.Contains(string(torOk), "250 OK") {
			fmt.Println("TOR authentication failed. Please, try again or restart Torwrapper")
			fmt.Println(string(torOk))
			return
		}

		_, err = torCon.Write([]byte("signal newnym \"\"\n"))
		check(err)
		_, err = torCon.Read(torOk)
		check(err)
		if !strings.Contains(string(torOk), "250 OK") {
			fmt.Println("TOR IP change failed. Please, try again or restart Torwrapper")
			return
		}

		fmt.Println("Changing IP address...")
		time.Sleep(8 * time.Second)

		fmt.Print("\n")
		checkIp()
	} else {
		fmt.Println("To change IP address Torwrapper should be enabled. Run \"torwrapper start\" to do so")
	}
}

// checkIp prints IP address via Get request to ident.me and Tor connection status
func checkIp() {
	torIpStdout, _ := execSh(CHECK_TOR_IP_CMD)
	if torIpStdout != "" {
		fmt.Printf("You are now connected to the TOR network\n"+
			"Your IP address: %s (according to https://check.torproject.org)\n", torIpStdout[:len(torIpStdout)-1])
		return
	}

	ipStdout, _ := execSh(CHECK_IP_CMD)
	if ipStdout != "" {
		fmt.Printf("You are NOT connected to the TOR network\n"+
			"Your IP address: %s (according to https://ident.me)\n", ipStdout)
		return
	}

	fmt.Println("IP identification request timeout: check your internet connection and firewall")
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

		fmt.Println("DNS has been changed to OpenNIC until reboot")
	} else {
		fmt.Println("To change DNS Torwrapper should be enabled. Run \"torwrapper start\" to do so")
	}
}

// updateBridges downloads recent private bridges from project's GitHub repo
func updateBridges() {
	stdout, stderr := execSh(DOWNLOAD_BRIDGES_CMD)

	if stderr != "" {
		fmt.Printf("Some problem occurred while connecting to the source:\n%s", stderr)
		return
	}

	bridgesRaw := strings.SplitAfter(stdout, "\n")
	bridges := "Bridge "
	bridges += strings.Join(bridgesRaw[:len(bridgesRaw)-1], "Bridge ")
	addTextIfAbsent("/etc/tor/bridges.txt", bridges, true)
	fmt.Println("Bridges have been updated successfully")
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
	case "changeid":
		restartTorService()
	case "myip":
		checkIp()
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
