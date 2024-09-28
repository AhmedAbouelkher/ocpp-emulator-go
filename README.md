# Dummy OCPP Charging Point in Golang

This is a dummy OCPP charging point implementation in Golang. It is a simple implementation that can be used to test OCPP central systems.

Example Usage:

Pull the repository and run the following command in the root directory of the repository:

```shell
go run *.go -cs "ws://localhost:8180/steve/websocket/CentralSystemService" -cp "59876295d63fa77be21a" -control-port "7123"
```
