# Torwrapper
_A small Tor-based proxy tool for Linux systems that automatically applies valid Bridges._

## Info

This tool was designed to redirect all the traffic (including DNS) to the Tor and block all the network connections with 
your system except those which are running through Tor.

Was tested on __Ubuntu__ and __Debian__.

## Usage
Almost every command should be executed with `sudo` privileges (e.g. `sudo torwrapper start`).

Use parameter `updbridges` if service cannot establish connection with the Tor network. Bridges will be downloaded from 
[https://torscan-ru.ntc.party/](https://torscan-ru.ntc.party/). 
If they don't work — visit [https://bridges.torproject.org](https://bridges.torproject.org) 
and copy custom bridges to /etc/tor/bridges.txt, after which try to start the service again.

Don't try to manage Torwrapper with `systemctl start/stop/restart torwrapper` — it won't work, because the service represents only the status of the tool (active/inactive) and does not perform any useful job! 

### Install
1. `git clone https://github.com/KostarevVI/torwrapper.git`
2. `cd torwrapper`
3. `sudo ./install.sh`

### List of allowed params
    start		Run Torwrapper for this system 
    stop		Stop Torwrapper and restore settings
    restart		Consequent launch of "stop" and "start"
    status		Check if Torwrapper is available (on/off state)
    changeid	TOR restart for identity (IP) change
    myip		Learn IP address
    dns		Change present DNS for OpenNIC DNS 
    updbridges	Update TOR bridges from the source web page

### Uninstall
`sudo ./uninstall.sh`
