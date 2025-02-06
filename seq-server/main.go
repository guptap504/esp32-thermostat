package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/goburrow/modbus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type SetRequest struct {
	Value int `json:"value"`
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

	err = handler.Connect()
	if err != nil {
		panic(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)

	e.GET("/read", func(c echo.Context) error {
		done := make(chan bool)
		requestQueue <- func() {
			results, err := client.ReadHoldingRegisters(1, 12)
			if err != nil {
				c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			response := make([]uint16, 0)
			for i := 0; i < len(results); i += 2 {
				r := binary.BigEndian.Uint16(results[i : i+2])
				response = append(response, r)
			}
			c.JSON(http.StatusOK, response)
			done <- true
		}
		<-done
		return nil
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
		requestQueue <- func() {
			_, err := client.WriteSingleRegister(uint16(address), uint16(request.Value))
			if err != nil {
				c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			done <- true
		}
		<-done
		return c.NoContent(http.StatusOK)
	})

	e.GET("/info", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"serial_number": config.SerialNumber})
	})

	e.Logger.Fatal(e.Start(":8000"))
}
