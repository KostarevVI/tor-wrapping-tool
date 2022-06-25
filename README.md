# torwrapper
## Info

This tool was developed to redicert all trafic (including DNS) to the Tor and block all the network connections with your system exept those which 
are running throught Tor.

## Usage
Almost every command should be executed with `sudo` priveledges (e.g. `sudo torwrapper start`).

Use parameater `updbridges` if service cannot establish connection with the Tor network. Bridges will be downloaded from 
[https://torscan-ru.ntc.party/](https://torscan-ru.ntc.party/). 
If they don't work — visit [https://bridges.torproject.org](https://bridges.torproject.org) 
and copy custom bridges to /etc/tor/bridges.txt, after which try to start the service again.

### Install
1. `git clone https://github.com/KostarevVI/torwrapper.git'
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
