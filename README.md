# Steps to build image

## Setup python libraries

```sh

# Create and activate virtualenv
pyenv virtualenv 3.10 esp32
pyenv activate esp32

python3 -m pip install esptool
python3 -m pip install mpremote
```

## Flash esp32

```sh
esptool.py erase_flash

esptool.py --baud 460800 write_flash 0x1000 firmware/ESP32_GENERIC-20241129-v1.24.1.bin
```

## Clone libraries

```sh
git clone https://github.com/miguelgrinberg/microdot

git clone https://github.com/brainelectronics/micropython-modbus
```

## Copy libraries into esp32 root

```sh
cp -R microdot/src/microdot code/

cp -R micropython-modbus/umodbus code/
```

## Setup the esp32

```sh
# Activate virualenv
# pyenv activate esp32

./flash.sh <ssid> <password>
``
```
