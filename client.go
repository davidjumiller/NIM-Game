package main

import (
	"encoding/json"
	"fmt"
	"github.com/DistributedClocks/tracing"
	"io/ioutil"
	"os"
	"strconv"
	"net"
)

/** Config struct **/

type ClientConfig struct {
	ClientAddress        string
	NimServerAddress     string
	TracingServerAddress string
	Secret               []byte
	TracingIdentity      string
}

/** Tracing structs **/

type GameStart struct {
	Seed int8
}

type ClientMove StateMoveMessage

type ServerMoveReceive StateMoveMessage

type GameComplete struct {
	Winner string
}

/** Message structs **/

type StateMoveMessage struct {
	GameState []uint8
	MoveRow   int8
	MoveCount int8
}

func main() {
	tracingServer := tracing.NewTracingServerFromFile("config/tracing_server_config.json")
	err := tracingServer.Open()
	if err != nil {
		return
	}
	defer tracingServer.Close()
	go tracingServer.Accept()

	if len(os.Args) != 2 {
		fmt.Println("Usage: client.go [seed]")
		return
	}
	arg, err := strconv.Atoi(os.Args[1])
	CheckErr(err, "Provided seed could not be converted to integer", arg)
	seed := int8(arg)

	config := ReadConfig("config/client_config.json")
	tracer := tracing.NewTracer(tracing.TracerConfig{
		ServerAddress:  config.TracingServerAddress,
		TracerIdentity: config.TracingIdentity,
		Secret:         config.Secret,
	})
	defer tracer.Close()

	trace := tracer.CreateTrace()
	trace.RecordAction(
		GameStart{
			Seed: seed,
		})

	nimServerResolved, err := net.ResolveUDPAddr("udp", config.NimServerAddress)
	if err != nil { return }
	nimClientResolved, err := net.ResolveUDPAddr("udp", config.ClientAddress)
	if err != nil { return }
	
	// Might need to retry if dial cant connect?
	conn, err := net.DialUDP("udp", nimClientResolved, nimServerResolved)
	if err != nil { return }
	defer conn.Close()
}

func ReadConfig(filepath string) *ClientConfig {
	configFile := filepath
	configData, err := ioutil.ReadFile(configFile)
	CheckErr(err, "reading config file")

	config := new(ClientConfig)
	err = json.Unmarshal(configData, config)
	CheckErr(err, "parsing config data")

	return config
}

func CheckErr(err error, errfmsg string, fargs ...interface{}) {
	if err != nil {
		fmt.Fprintf(os.Stderr, errfmsg, fargs...)
		os.Exit(1)
	}
}
