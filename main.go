// amqp_client is a simple program that communicates with Bear Robotics
// AMQP-based mission service via a RabbitMQ server.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	amqpgo "github.com/rabbitmq/amqp091-go"
	"gitlab.com/sajjad7/bear-api-client/amqp"
)

const (
	apiHelp = `
JSON message has the follwing fields:
{
  "cmd": API function,
  "args": {} // optional args
}

where available API functions are:
  /api/2/get/destinations to list destination.
  /api/2/get/mission/status to get mission status.
  /api/2/get/overview to get service overview.
  /api/2/get/robot/status to get robot status.
  /api/2/get/trays/status to get trays status.
  /api/2/post/mission/new to post a new mission.
  /api/2/post/mission/cancel to cancel the current mission.
`
)

// Command represents a function used in RPCs. This  is only used by client
// to check validity of user input functions since raw message is sent to the
// remote service.
type Command struct {
	Cmd  string
	Args map[string]interface{}
}

func main() {
	command := flag.String("func", "{}", fmt.Sprintf("Function to call on robot in RPC mode. %s", apiHelp))
	mode := flag.String("mode", "", "Client mode, should be either rpc or subscribe")
	rmqPass := flag.String("rmq_pass", "guest", "RabbitMQ user password")
	rmqUser := flag.String("rmq_user", "guest", "RabbitMQ user login")
	rmqServerAddr := flag.String("rmq_addr", "localhost:5672", "RabbitMQ server address")
	robotID := flag.String("robot_id", "", "Robot id, e.g., pennybot-abc123")
	rmqVHost := flag.String("rmq_vhost", "/", "RMQ virtual host")
	certPath := flag.String("cert_path", "", "Path containing SSL certificates.")

	flag.Parse()
	// Check sanity of user inputs.
	if *robotID == "" {
		log.Panicln("-robot_id flag is required.")
	}
	if *mode != "rpc" && *mode != "subscribe" {
		log.Panicln("-mode should be either rpc or subscribe.")
	}
	jsonCommand := json.RawMessage(*command)
	var c Command
	err := json.Unmarshal(jsonCommand, &c)
	if err != nil {
		log.Panicf("-func value is not a valid JSON/function message: %v.", err)
	}

	var conn *amqpgo.Connection
	if *certPath == "" {
		conn, err = amqp.Dial(*rmqUser, *rmqPass, *rmqServerAddr, *rmqVHost)
	} else {
		conn, err = amqp.DialTLS(*rmqServerAddr, *rmqVHost, *certPath)
	}

	if err != nil {
		log.Panicf("Failed to connect to RabbitMQ server: %v.", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Panicf("Failed to open a channel: %v.", err)
	}
	defer ch.Close()

	if *mode == "rpc" {
		caller := amqp.NewCaller(ch)
		res, err := caller.Call(*robotID, []byte(*command), time.Second*3)
		if err != nil {
			log.Printf("Failed to handle RPC request: %v.", err)
		} else {
			log.Println(string(res))
		}
	} else {
		// *mode == subscribe
		missionSub := amqp.NewSubscriber(ch)
		missionUpdates, err := missionSub.Subscribe(*robotID, *robotID+"/mission_update")
		if err != nil {
			log.Printf("Faild to consume topic: %v.", err)
		}
		defer func() {
			err = missionSub.Unsubscribe()
			if err != nil {
				log.Printf("Failed to unsubscribe: %v", err)
			}
		}()

		traySub := amqp.NewSubscriber(ch)
		trayUpdates, err := traySub.Subscribe(*robotID, *robotID+"/trays_update")
		if err != nil {
			log.Printf("Faild to consume topic: %v.", err)
		}
		defer func() {
			err = traySub.Unsubscribe()
			if err != nil {
				log.Printf("Failed to unsubscribe: %v", err)
			}
		}()

		for {
			select {
			case msg := <-missionUpdates:
				log.Printf("New mission update: %s.", string(msg.Body))
			case msg := <-trayUpdates:
				log.Printf("New tray update: %s.", string(msg.Body))
			}
		}
	}
}
