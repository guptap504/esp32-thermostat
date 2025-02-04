#!/bin/bash

set -e

# Check if serial number and Wi-Fi credentials are provided
if [ -z "$1" ] || [ -z "$2" ] ; then
  echo "Usage: $0 <wifi_ssid> <wifi_password>"
  exit 1
fi

SERIAL_NUMBER=$(python random_string.py) 
TOKEN=$(python random_string.py)
WIFI_SSID=$1
WIFI_PASSWORD=$2

# Create device_config.py with the serial number
echo "SERIAL_NUMBER = '$SERIAL_NUMBER'" >  device.py
echo "THERMOSTAT_ADDRESS = 1" >> device.py

# Create wifi_credentials.py with the Wi-Fi credentials
echo "WIFI_SSID = '$WIFI_SSID'" > wifi.py
echo "WIFI_PASSWORD = '$WIFI_PASSWORD'" >> wifi.py

# Create auth.py with the token
echo "TOKEN = '$TOKEN'" > auth.py

mv device.py wifi.py auth.py code/config/

mkdir final

cp code/boot.py final/
cp code/main.py final/

python mpy_cross_all.py code/config -o final/config
python mpy_cross_all.py code/umodbus -o final/umodbus
mpy-cross-v6.3 code/microdot.py -o final/microdot.mpy

python -m mpremote cp final/boot.py : + cp final/main.py : + cp -rf final/config  : + cp -rf final/umodbus : + cp final/microdot.mpy : + reset


# Clean up
rm -rf final

sleep 1

IP=$(python -m mpremote exec 'print(sta_if.ifconfig())')

echo "Serial Number: $SERIAL_NUMBER ; Token: $TOKEN; WIFI_IP: $IP"
