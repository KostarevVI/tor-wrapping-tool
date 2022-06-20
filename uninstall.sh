#!/bin/bash

export DEBIAN_FRONTEND=noninteractive

if systemctl --all | grep -Fq 'torwrapper'; then
	echo "Torwrapper is enabled. Stopping..."
	systemctl stop torwrapper
fi

rm /etc/systemd/system/torwrapper.service
rm /usr/bin/torwrapper
rm /etc/tor/bridges.txt

echo "Torwrapper uninstalled"