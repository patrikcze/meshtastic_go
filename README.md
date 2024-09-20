# Meshtastic Go

Meshtastic Go is a Go application for interacting with Meshtastic devices over a serial connection. This project allows you to send and receive messages, configure settings, and manage nodes in a Meshtastic network.

## Features

- Send and receive text messages
- Manage device configurations
- Support for multiple platforms (Windows, Linux, macOS)
- Linting and security checks integrated into the build process

## Prerequisites

- Go 1.16 or later
- Go modules enabled (`GO111MODULE=on`)

## Installation

Clone the repository:

```bash
git clone https://github.com/yourusername/meshtastic_go.git
cd meshtastic_go
```

Install the necessary dependencies:

```bash
go mod tidy
```

## Build

To build the application for all platforms, run:

```bash
make
```

You can also build for specific targets by running:

```bash
make build
```

## Linting

To run linting checks on the code, use:

```bash
make lint
```

## Usage

After building, you can run the application:

```bash
./bin/meshtastic_go_linux_amd64  # Linux example
./bin/meshtastic_go_windows_amd64.exe  # Windows example
./bin/meshtastic_go_darwin_amd64  # macOS example
```

## Contributing

If you wish to contribute to this project, please fork the repository and create a pull request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Meshtastic](https://meshtastic.org/) for their open-source projects and support.