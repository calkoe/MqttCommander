[![goreleaser](https://github.com/calkoe/MqttCommander/actions/workflows/build.yml/badge.svg)](https://github.com/calkoe/MqttCommander/actions/workflows/build.yml)

![Logo](https://user-images.githubusercontent.com/26646066/138765958-a8a80327-2f55-478b-991e-bc5523d5a2f1.png)
![IMG_0EC4D6E1BBAC-1](https://user-images.githubusercontent.com/26646066/141022858-2f78ebd0-f7a2-4eee-afb0-aac1657fe2fe.jpeg)


# ⚙ MQTT COMMANDER - A fast, robust and easy to use MQTT rule engine

## How does ist work an what can i do with it ?

The principle is simple: <br>
a automations consists of two components: constraints and actions. <br>
If all constraints are met, the configured actions are triggered.

### Example:
```yml
  - Name: My Awesome Example Rule
    Constraints: 
      - Mqtt : livingRoom/sensor.temperature >= 25
    Actions:
      - Mqtt : relais/fan.on = 1
      - Mqtt : system/notification = "Too hot 🔥! Fan has been tunred on 🔌!"
```
Of course, you can make the rules more complex and use additional conditions. These are described below.

The rules are saved in ".yml" files. These are saved in the "Automations /" folder. When the software is started for the first time, an example file is created there. Nested sub-folders are also possible.

**Changes to the files are automatically transferred to the live system. ✅**<br/>
(Attention: If a ".yml" is changed, all the rules contained therein are restarted)

--- 

## Features
* Supports JSON encoded MQTT Messages (e.g. **mytopic/sensor.value**)
* Supports all common comparators [<,>,<=,>=,==,!=]
* Supports RegEx in String compararison
* Supports Cron triggered events
* Supports HTTP calls
* LIVE reload of config files
* Templates
* Special functions, like Timeout, reminder and auto reset of rules
* Web-Dashbaord ro review the Status of your Rules 
* Focus on reliability and speed

--- 

## Getting Started:

QuickStart on Docker:
```
docker run -it -p 9090:9090 -v mqttcommanderdata:/data --name mqttcommander calkoe/mqttcommander
```
Note: Your Config files will be stored at the "mqttcommanderdata" volume.

Use one of this pre-Compiled Binarys, at the first start an order with the name "Config" is automatically created, here you will also find some sample files.

[Download MqttCommander](https://github.com/calkoe/MqttCommander/releases)

Of course, you can also start or compile the program yourself after downloading this repository.

```bash
go run MqttCommander
```

```bash
go build MqttCommander
```

When the software is started for the first time, the configuration folder structure is created in the same directory as the executable file.

Enter the URI of your MQTT server in the "config.yml" file.

After restarting the software, you will find an overview of the active automations at http://localhost:9090 🔥

--- 
## Possible rule options:
|Option          |Required |Default |Values         |Description
|---             |---      |---     |---            |---          
|```Name```      |✅       |-        | TEXT          |Humand readable name of your Automation
|```Triggered``` |❌       |false    | true / false  |Defines whether the rule is set by default
|```Hidden```    |❌       |false    | true / false  |Hide these rules in the dashboard
|```Mode```      |❌       |AND      | AND / OR      |How many conditions must be met
|```Retrigger``` |❌       |false    | true / false  |Execute the actions again without the rule having to be inactive beforehand (set 0 to deactivate)
|```Pause```     |❌       |0s       | duration      |Minimum time between triggering the rule (set 0 to deactivate)
|```Delay```     |❌       |0s       | duration      |Delay between the fulfilment of the conditions and the triggering of the actions (set 0 to deactivate)
|```Reminder```  |❌       |0s       | duration      |Adjustable time interval in which the actions are repeated as long as all conditions are fulfilled (set 0 to deactivate)
|```Constraint```|❌       |-        | Constraints[] |List of all conditions
|```Actions```   |❌       |-        | Actions[]     |List of all actions

## Available constraints:
|Identifier          |Syntax Example                                        |Description
|---                 |---                                                   |---          
|```Cron```          | ```* * * * * * * (Reset 2s)```                       |Time condition in the usual CRON format, supports seconds and time span
**Possible options:**
Reset                | Reset [duration])                                    | resets the condition after a defined period of time (Set 0 to disable, Default: 0)
NoTrigger            | NoTrigger [0/1]                                      | actions are not triggered if this condition fulfils the mode of the rule (Default: 0)
|```Mqtt```          | ```demo/sensor.value <= "3" (Reset 2s)```            | Received values from MQTT topics, JSON objects and various data types are possible
**Possible options:**
Reset                | Reset [duration])                                    | resets the condition after a defined period of time (Set 0 to disable, Default: 0)
NoTrigger            | NoTrigger [0/1]                                      | actions are not triggered if this condition fulfils the mode of the rule (Default: 0)
Timeout              | Timeout [duration]                                   | Condition is fulfilled if no more data has been received since the set time period (Set 0 to disable, Default: 0)
BlockRetained        | BlockRetained [0/1]                                  | Discard retained MQTT messages (Default: 0)
NoValue              | NoValue [0/1]                                        | Do not set the rule content to the message content (Default: 0)

## Available actions:
|Identifier          |Syntax Example                                        |Description
|---                 |---                                                   |---          
|```Http```          | ```https://my-server.com/restapi (Reverse 2s)```     |Call specify URL 
**Possible options:**
Reverse              | Reverse [0/1]                                        | This action is only executed when the rule is reset (Default: 0)
|```Mqtt```          | ```demo/sensor.value = "3" (Retained 1)```           | Send messages to the MQTT Broker, JSON objects, templates and various data types are possible
**Possible options:**
Reverse              | Reverse [0/1]                                        | This action is only executed when the rule is reset (Default: 0)
Retained             | Retained [0/1]                                        | Retain message (Default: 0)

**Note: It is possible to insert placeholders in the message content of the actions, these are e.g. {{.Name}} for the name of the rule or {{.Value}} for the value of the rule.**

## Example:
```yml
- Name: Full Demo MQTT Automation
  Mode: AND
  Retrigger: true
  Pause: 10s
  Delay: 0s
  Reminder: 1m
  Constraints: 
    - Mqtt : demo/sensor.value <= "3" (Reset 2s)  (Timeout 5s) (BlockRetained 0)
  Actions:
    - Mqtt : demo/actuator = 1 (Retained 0)
    - Mqtt : demo/notification = {{.Name}} triggered! 🤓
```
---
## MqttCommander Roadmap:
- [x] Add Support for Multi-Level JSON Objects
- [x] Complete Documentation
- [ ] Dashbaord: Add indicator for ivalid rules
- [ ] Dashbaord: Display Timeout and Reset as optional badges
- [ ] Dashbaord: Fix representation of long numbers
- [ ] Add Support for XOR Rules
- [ ] Dashbaord: Redesign File-Overview

---

## Often used together:
- EMQX: https://github.com/emqx/emqx
- Zigbee2Mqtt: https://github.com/Koenkk/zigbee2mqtt
- Node-Red: https://github.com/node-red/node-red

**docker-compose.yml**
```yml
emqx:
  container_name: emqx
  image: emqx/emqx
  ports:
    - 1883:1883
    - 8883:8883
    - 18083:18083
  volumes:
    - emqx_opt_emqx_data:/opt/emqx/data
    - emqx_opt_emqx_etc:/opt/emqx/etc
    - emqx_opt_emqx_log:/opt/emqx/log
  restart: unless-stopped

zigbee2mqtt:
  container_name: zigbee2mqtt
  image: koenkk/zigbee2mqtt
  ports:
    - 8888:8888
  volumes:
    - zigbee2mqtt_app_data:/app/data
  devices:
    - /dev/USBZigBee:/USBZigBee
  links:
    - emqx
  depends_on:
    - emqx
  restart: unless-stopped

node-red:
  container_name: node-red
  image: nodered/node-red
  network_mode: host
  volumes:
    - node-red_data:/data
  depends_on:
    - emqx
  restart: unless-stopped

mqttcommander:
  container_name: mqttcommander
  image: calkoe/mqttcommander
  ports:
    - 9090:9090
  volumes:
    - mqttcommander_data:/data
  links:
    - emqx
  depends_on:
    - zigbee2mqtt
    - node-red
  restart: unless-stopped
```
