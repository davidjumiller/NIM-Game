package main

import (
	"encoding/json"
	"encoding/gob"
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

	// Address resolution
	nimServerResolved, err := net.ResolveUDPAddr("udp", config.NimServerAddress)
	CheckErr(err, "Cannot resolve server address")
	nimClientResolved, err := net.ResolveUDPAddr("udp", config.ClientAddress)
	CheckErr(err, "Cannot resolve client address")
	
	// Create connection to nim server
	// Might need to retry if dial cant connect?
	conn, err := net.DialUDP("udp", nimClientResolved, nimServerResolved)
	CheckErr(err, "Cannot dial connection")
	defer conn.Close()

	messageOut := StateMoveMessage{nil, -1, 32}

	// Encode
	enc := gob.NewEncoder(conn)
	err = enc.Encode(StateMoveMessage{nil, -1, 32})
	CheckErr(err, "Error with encoding/sending message")
	trace.RecordAction(
		ClientMove{
			GameState: messageOut.GameState,
			MoveRow: messageOut.MoveRow,
			MoveCount: messageOut.MoveCount,
		})

	// Decode
	messageIn := new(StateMoveMessage)
	dec := gob.NewDecoder(conn)
	err = dec.Decode(messageIn)
	CheckErr(err, "Error with decoding/receiving message")
	trace.RecordAction(
		ServerMoveReceive{
			GameState: messageIn.GameState,
			MoveRow: messageIn.MoveRow,
			MoveCount: messageIn.MoveCount,
		})

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
