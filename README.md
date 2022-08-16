# Bear API Client
This is a simple client program to demonstrate using Bear Robotics mission API using AMQP protocol. This implementation is done in GoLang but clients can implemented by any programming language as long as they follow the AMQP protocol.

## Getting the client
The client can be installed using gotool. This command will install the binary in `$GOPATH/bin`.
```sh
go install gitlab.com/bearrobotics-public/api-client
```



## Running the client
To run the client, we need to provide the robot Id that we wish to control/monitor, as well as rabbitmq server address information and authentication method. You can use -help flag to see an overview of the supported parameters.

The client can run in two modes: (1) RPC when it sends a command to the robot and prints the response on the console, and (2) subscribe mode where it subscribes to events from a particular robot and print any change in robot status on the console.

```sh
 $ api-client -help
  -cert_path string
        Path containing SSL certificates.
  -func string
        Function to call on robot in RPC mode.
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
         (default "{}")
  -mode string
        Client mode, should be either rpc or subscribe
  -rmq_addr string
        RabbitMQ server address (default "localhost:5672")
  -rmq_pass string
        RabbitMQ user password (default "guest")
  -rmq_user string
        RabbitMQ user login (default "guest")
  -rmq_vhost string
        RMQ virtual host (default "/")
  -robot_id string
        Robot id, e.g., pennybot-abc123

```

### Authenticate with plain text
To authenticate the client using user/password use `rmq_user` and `rmq_pass` flags.
```sh
api-client -robot_id pennybot_abc123 -mode subscribe -rmq_user user -rmq_pass -rmq_addr serverAddr:5672 -rmq_vhost vhost
```
### Authenticate with certificates
To authenticate the client with certifciates use `cert_path` pointing to folder where they are stored.
```sh
api-client -robot_id pennybot_abc123 -mode subscribe -rmq_addr serverAddr:5671 -cert_path path -rmq_vhost vhost
```

### Sending commands to tray mission service
The Client can send commands to the mission service running on the robot in the form of remote procedure call (RPC). To do that client sends a command message to `$robot_id` exchnage on the RabbitMQ server and wait for the response from the robot. Server will transfer the message to the robot and will redirect its response to client when it arrives. Matching the response with the right request is done using two message fields "Reply To" (where) and "Correlation ID" (which).

`Call()` function in `amqp/amqp.go` abstracts this sequence of operations.

#### Sample commands:
1. Getting lists of destinations
```sh
 api-client  -mode rpc -func '{"cmd": "/api/2/get/destinations"}'
```
2. Sending a new mission
```sh
 api-client -mode rpc -func\
 '{"cmd": "/api/2/post/mission/new", "args": {"destinations": ["T1", "T2"], \
 "trays": [{"name": "top", "destination": "T1"}, {"name": "middle", "destination": "T2"}], "mode": "Serving"}}'
```
3. Getting mission status
```sh
api-client -mode rpc -func '{"cmd": "/api/2/get/mission/status"}'
```
4. Canceling the current mission
```sh
api-client -mode rpc -func '{"cmd": "/api/2/post/mission/cancel"}'
```


### Getting mission/tray updates
RabbitMQ server manages the message delivery using various exchanges. In a working communication, both sender and receiver must know the exchange topology in advance.

In the current implementation the clients can register callback to following exchanges that publish robot/mission updates:
1. `$robot_id/mission_update` for updates to current mission status
2. `$robot_id/trays_update` for updates to trays status
