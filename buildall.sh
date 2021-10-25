env GOOS=darwin GOARCH=amd64 go build -o MqttCommander_darwin_amd64 MqttCommander
env GOOS=linux GOARCH=amd64 go build -o MqttCommander_linux_amd64 MqttCommander
env GOOS=linux GOARCH=arm GOARM=5 go build -o MqttCommander_linux_armv5 MqttCommander
env GOOS=windows GOARCH=amd64 go build -o MqttCommander_windows_amd64.exe MqttCommander