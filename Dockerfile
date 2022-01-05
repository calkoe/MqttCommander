FROM debian
COPY MqttCommander /

EXPOSE 9090/tcp
VOLUME data
ENV MQTTC_CONFIG_PATH=data
ENTRYPOINT ["/MqttCommander"]