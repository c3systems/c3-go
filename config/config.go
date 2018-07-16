package config

import (
	"fmt"
	"time"
)

// ServerHost ...
const ServerHost = "0.0.0.0"

// ServerPort ...
const ServerPort = 3333

// DefaultStoreDirectory is the default directory where the file system store will live.
var DefaultStoreDirectory = "~/.c3"

// TempContainerStatePath ...
var TempContainerStatePath = "/tmp"

// TempContainerStateFileName ...
var TempContainerStateFileName = "state.json"

// TempContainerStateFilePath ...
var TempContainerStateFilePath = fmt.Sprintf("%s/%s", TempContainerStatePath, TempContainerStateFileName)

// DockerRegistryPort ...
const DockerRegistryPort = 5000

// IPFSGateway ...
const IPFSGateway = "http://127.0.0.1:9001"

// BlockDifficulty ...
const BlockDifficulty = 5

// MinedBlockVerificationTimeout ...
const MinedBlockVerificationTimeout = 10 * time.Minute
