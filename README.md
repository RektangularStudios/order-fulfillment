# Order Fulfillment

Microservice to create and fulfill orders made on Novellia

### Execution

Install Go: https://golang.org/doc/install

The first argument is the `configPath` which determines a YAML file to load. The choices for these are in `config/`.

Start the server without building
- `go run ./server/* ${PWD}/config/local.yaml`

Or compile a binary for deployment
- `go build -o order-fulfillment-server ./server/*`

Then just execute binary to start the server
- `./order-fulfillment-server ${PWD}/config/local.yaml`

### Executing Mock Server

Same as above, except you don't need a real instance to supply all the depended services. A `mocked` switch can be set within the configuration YAML.

Start mock server (after building)
- `./order-fulfillment-server ${PWD}/config/mock.yaml`
