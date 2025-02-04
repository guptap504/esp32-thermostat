import network
from config.wifi import WIFI_SSID, WIFI_PASSWORD
from config.device import SERIAL_NUMBER

sta_if = network.WLAN(network.STA_IF)
sta_if.active(True)
sta_if.config(dhcp_hostname=f"th-{SERIAL_NUMBER}")
sta_if.connect(WIFI_SSID, WIFI_PASSWORD)

print("Wi-Fi Connected:", sta_if.ifconfig())
