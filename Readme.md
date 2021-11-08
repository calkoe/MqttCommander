![Logo](https://user-images.githubusercontent.com/26646066/138765958-a8a80327-2f55-478b-991e-bc5523d5a2f1.png)

# ⚙ MQTT COMMANDER - A robust and easy to use MQTT rule engine

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

The rules are saved in ".yml" files. These are saved in the "Automations /" folder. When the software is started for the first time, an example file is created there. Sub-folders are also possible.

**Changes to the files are automatically transferred to the live system. ✅**<br/>
(Attention: If a ".yml" is changed, all the rules contained therein are restarted)

<hr>

## Features
* Supports JSON encoded MQTT Messages (e.g. **mytopic/sensor.value**)
* Supports all common comparators [<,>,<=,>=,==,!=]
* Supports Cron triggered events
* Supports HTTP calls
* Supports LIVE reload of config files
* Special functions, like Timeout, reminder and auto reset of rules
* Simple Web-Dashbaord ro review the Status of your Rules 

<hr>

## Getting Started:
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

<img width="1440" alt="image" src="https://user-images.githubusercontent.com/26646066/140827939-a971d086-9b35-49ca-bb19-cb93baa41715.png">

<hr>

## Create more complex rules:
```yml
  - Name: Full Demo MQTT Automation
    Mode: AND
    Retrigger: true
    Pause: 10s
    Delay: 10s
    Reminder: 1m
    Constraints: 
      - Mqtt : demo/sensor.value <= "3" -Reset 2s  -Timeout 5s -BlockRetained 0
    Actions:
      - Mqtt : demo/actuator = 1 -Retained 0
```
Most of the functions are self-explanatory based on the examples, but detailed documentation will follow soon
