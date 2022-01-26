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
	"bytes"
	"reflect"
	"time"
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

	messageOut := StateMoveMessage{nil, -1, seed}
	messageIn := messageOut
	gameDone := false
	var winner string
	for gameDone == false {
		for i := 0; i < len(messageOut.GameState); i++{
			if (messageOut.GameState[i] > 0) {
				break
			} else if (i >= len(messageOut.GameState)-1) {
				gameDone = true
				winner = "server"
			}
		}
		SendMessage(messageOut, conn)
		trace.RecordAction(
			ClientMove{
				GameState: messageOut.GameState,
				MoveRow: messageOut.MoveRow,
				MoveCount: messageOut.MoveCount,
			})
		if gameDone == false {
			messageTemp := ReceiveMessage(&messageIn, conn)
			if messageTemp.GameState != nil {
				trace.RecordAction(
					ServerMoveReceive{
						GameState: messageTemp.GameState,
						MoveRow: messageTemp.MoveRow,
						MoveCount: messageTemp.MoveCount,
					})
			}

			gameDone = makeMove(&messageOut, messageIn)
			if gameDone == true { winner = "client" }
		}
	}
	trace.RecordAction(GameComplete{winner})
}

func SendMessage(msg StateMoveMessage, conn net.Conn) {
	var oBuf bytes.Buffer
	enc := gob.NewEncoder(&oBuf)
	err := enc.Encode(msg)
	CheckErr(err, "Error with encoding message")
	_, err = conn.Write(oBuf.Bytes())
	CheckErr(err, "Error with sending message")
}

func ReceiveMessage(messageIn *StateMoveMessage, conn net.Conn) StateMoveMessage {
	var messageTemp StateMoveMessage
	conn.SetReadDeadline(time.Now().Add(time.Second))
	dec := gob.NewDecoder(conn)
	err := dec.Decode(&messageTemp)
	if err == nil {
		*messageIn = messageTemp
	}
	return messageTemp
}

func makeMove(messageOut *StateMoveMessage, messageIn StateMoveMessage) bool {
	// Check if server made proper move based on previous message out by client
	if (messageIn.MoveRow != -1) {
		messageTemp := *messageOut
		messageTemp.GameState[messageIn.MoveRow] -= uint8(messageIn.MoveCount)
		if !reflect.DeepEqual(messageTemp.GameState, messageOut.GameState) {
			return false
		}
	}
	// Client move "logic"
	*messageOut = messageIn 
	for i := 0; i < len(messageOut.GameState); i++{
		if (messageOut.GameState[i] > 0) {
			messageOut.GameState[i]--
			messageOut.MoveRow = int8(i)
			messageOut.MoveCount = 1
			i = len(messageOut.GameState)
		} else if (i >= len(messageOut.GameState)-1) {
			return true
		}
	}
	return false
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
