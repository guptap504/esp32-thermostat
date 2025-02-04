import json

from config.device import SERIAL_NUMBER
from config.auth import TOKEN
from microdot import Microdot
from umodbus.serial import Serial as ModbusRTUMaster

modbus = ModbusRTUMaster(
    pins=(17, 16),  # given as tuple (TX, RX), check MicroPython port specific syntax
    baudrate=9600,  # optional, default 9600
    data_bits=8,  # optional, default 8
    stop_bits=1,  # optional, default 1
    parity=None,  # optional, default None
    ctrl_pin=32,  # optional, control DE/RE
    uart_id=1,  # optional, see port specific documentation
)

# Initialize HTTP Server
app = Microdot()

# Thermostat Address
THERMOSTAT_ADDRESS = 1


# Endpoint: GET /read
@app.route("/read", methods=["GET"])
def read_thermostat(request):
    token = request.headers.get("Authorization")
    if token != TOKEN:
        return {"error": "Unauthorized"}, 401
    try:
        registers = modbus.read_holding_registers(slave_addr=THERMOSTAT_ADDRESS, starting_addr=1, register_qty=12)
        return json.dumps(registers)
    except Exception as e:
        return {"error": str(e)}, 500


# Endpoint: PUT /set
@app.route("/set/<register>", methods=["PUT"])
def set_thermostat(request, register):
    token = request.headers.get("Authorization")
    if token != TOKEN:
        return {"error": "Unauthorized"}, 401
    try:
        data = request.json
        if "value" not in data:
            return {"error": "Value not provided"}, 400
        modbus.write_single_register(
            slave_addr=THERMOSTAT_ADDRESS, register_address=int(register), register_value=data["value"]
        )
        return {"status": "Value set successfully"}
    except Exception as e:
        return {"error": str(e)}, 500


# Endpoint: GET /info
@app.route("/info", methods=["GET"])
def get_info(request):
    token = request.headers.get("Authorization")
    if token != "IHLWjIyX5Zeo5uA":
        return {"error": "Unauthorized"}, 401
    return {"serial_number": SERIAL_NUMBER}


app.run(port=80)
