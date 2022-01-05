FROM ubuntu
RUN apt-get update
RUN apt-get install tzdata -y 
COPY MqttCommander /

EXPOSE 9090/tcp
VOLUME data
ENV MQTTC_CONFIG_PATH=data
ENTRYPOINT ["/MqttCommander"]