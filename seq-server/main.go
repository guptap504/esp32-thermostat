package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/goburrow/modbus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/stianeikeland/go-rpio/v4"
)

type SetRequest struct {
	Value int `json:"value"`
}

type UnoccupiedRequest struct {
	FanState int `json:"fanState"`
	Setpoint int `json:"setpoint"`
}

type Config struct {
	Port         string `json:"port"`
	SerialNumber string `json:"serialNumber"`
	AuthKey      string `json:"authKey"`
	Modbus       struct {
		BaudRate    int    `json:"baudRate"`
		DataBits    int    `json:"dataBits"`
		Parity      string `json:"parity"`
		StopBits    int    `json:"stopBits"`
		SlaveID     uint8  `json:"slaveId"`
		TimeoutSecs int    `json:"timeoutSecs"`
	} `json:"modbus"`
	Unoccupied struct {
		Pin      int `json:"pin"`
		FanState int `json:"fanState"`
		Setpoint int `json:"setpoint"`
	}
}

func ReadConfig(path string) (Config, error) {
	var config Config
	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return config, err
	}
	if config.Port == "" {
		config.Port = "/dev/ACM0"
	}
	if config.SerialNumber == "" {
		return config, errors.New("serialNumber is required")
	}
	if config.AuthKey == "" {
		config.AuthKey = "HYRJRDWMPEWE"
	}

	if config.Modbus.BaudRate == 0 {
		config.Modbus.BaudRate = 9600
	}
	if config.Modbus.DataBits == 0 {
		config.Modbus.DataBits = 8
	}
	if config.Modbus.Parity == "" {
		config.Modbus.Parity = "N"
	}
	if config.Modbus.StopBits == 0 {
		config.Modbus.StopBits = 1
	}
	if config.Modbus.SlaveID == 0 {
		config.Modbus.SlaveID = 1
	}
	if config.Modbus.TimeoutSecs == 0 {
		config.Modbus.TimeoutSecs = 5
	}
	if config.Unoccupied.Pin == 0 {
		config.Unoccupied.Pin = 27
	}
	if config.Unoccupied.FanState == 0 {
		config.Unoccupied.FanState = 1
	}
	if config.Unoccupied.Setpoint == 0 {
		config.Unoccupied.Setpoint = 23
	}

	return config, nil
}

var requestQueue = make(chan func(), 1000)

func main() {
	configPath := flag.String("config", "config.json", "Path to config file")
	flag.Parse()

	config, err := ReadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	e := echo.New()
	e.Use(middleware.KeyAuth(func(key string, c echo.Context) (bool, error) {
		return key == config.AuthKey, nil
	}))
	e.Use(middleware.CORS())
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	go func() {
		for request := range requestQueue {
			request()
		}
	}()

	handler := modbus.NewRTUClientHandler(config.Port)
	handler.BaudRate = config.Modbus.BaudRate
	handler.DataBits = config.Modbus.DataBits
	handler.Parity = config.Modbus.Parity
	handler.StopBits = config.Modbus.StopBits
	handler.SlaveId = config.Modbus.SlaveID
	handler.Timeout = time.Duration(config.Modbus.TimeoutSecs) * time.Second

	for {
		fmt.Println("trying to connect to ", config.Port)
		err = handler.Connect()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	defer handler.Close()
	err = rpio.Open()
	if err != nil {
		panic(err)
	}
	pin := rpio.Pin(config.Unoccupied.Pin)
	pin.Input()
	pin.Detect(rpio.AnyEdge)

	isOcccupied := false

	client := modbus.NewClient(handler)
	var previousFanState uint16
	var previousSetpoint uint16
	go func() {
		for {
			if pin.EdgeDetected() {
				res := pin.Read()
				if res == rpio.Low {
					requestQueue <- func() {
						results, err := client.ReadHoldingRegisters(3, 2)
						if err != nil {
							e.Logger.Warnf("Error reading from thermostat: %v", err)
						}
						previousSetpoint = binary.BigEndian.Uint16(results[:2])
						previousFanState = binary.BigEndian.Uint16(results[2:])
						_, err = client.WriteSingleRegister(uint16(3), uint16(config.Unoccupied.Setpoint))
						if err != nil {
							e.Logger.Warnf("Error writing to thermostat: %v", err)
						}
						_, err = client.WriteSingleRegister(uint16(4), uint16(config.Unoccupied.FanState))
						if err != nil {
							e.Logger.Warnf("Error writing to thermostat: %v", err)
						}
					}
					isOcccupied = false
				} else {
					requestQueue <- func() {
						if previousSetpoint != 0 && previousFanState != 0 {
							_, err := client.WriteSingleRegister(uint16(3), uint16(previousSetpoint))
							if err != nil {
								e.Logger.Warnf("Error writing to thermostat: %v", err)
							}
							_, err = client.WriteSingleRegister(uint16(4), uint16(previousFanState))
							if err != nil {
								e.Logger.Warnf("Error writing to thermostat: %v", err)
							}
						}
					}
					isOcccupied = true
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	e.GET("/read", func(c echo.Context) error {
		response := make([]uint16, 0, 12)
		results, err := client.ReadHoldingRegisters(1, 12)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		} else {
			for i := 0; i < len(results); i += 2 {
				r := binary.BigEndian.Uint16(results[i : i+2])
				response = append(response, r)
			}
		}
		return c.JSON(http.StatusOK, response)
	})

	e.POST("/set/:address", func(c echo.Context) error {
		address, err := strconv.ParseInt(c.Param("address"), 10, 16)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		var request SetRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		done := make(chan bool)
		var e error
		requestQueue <- func() {
			_, err := client.WriteSingleRegister(uint16(address), uint16(request.Value))
			if err != nil {
				e = err
			}
			done <- true
		}
		<-done
		if e != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": e.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "Value set successfully"})
	})

	e.GET("/info", func(c echo.Context) error {
		occ := "yes"
		if !isOcccupied {
			occ = "no"
		}
		return c.JSON(http.StatusOK, map[string]string{"serial_number": config.SerialNumber, "is_occupied": occ})
	})

	e.POST("/unoccupied", func(c echo.Context) error {
		var request UnoccupiedRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		config.Unoccupied.FanState = request.FanState
		config.Unoccupied.Setpoint = request.Setpoint
		file, err := os.OpenFile(*configPath, os.O_WRONLY, 0o644)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer file.Close()
		encoder := json.NewEncoder(file)
		err = encoder.Encode(config)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "updated config"})
	})

	e.Logger.Fatal(e.Start(":8000"))
}
