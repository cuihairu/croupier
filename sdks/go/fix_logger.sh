#!/bin/bash

# Fix all log calls to use c.logger or appropriate instance

# Fix client.go
sed -i '' 's/logInfof("Successfully connected and registered with Agent")/c.logger.Infof("Successfully connected and registered with Agent")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Local service address: %s")/c.logger.Infof("Local service address: %s")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Session ID: %s")/c.logger.Infof("Session ID: %s")/g' pkg/croupier/client.go
sed -i '' 's/logWarnf("Failed to register capabilities: %v")/c.logger.Warnf("Failed to register capabilities: %v")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Croupier client service started")/c.logger.Infof("Croupier client service started")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Registered functions: %d")/c.logger.Infof("Registered functions: %d")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Use Stop() to stop the service")/c.logger.Infof("Use Stop() to stop the service")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("===============================================")/c.logger.Infof("===============================================")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Service stopped by Stop() call")/c.logger.Infof("Service stopped by Stop() call")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Service stopped by context cancellation")/c.logger.Infof("Service stopped by context cancellation")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Service has stopped")/c.logger.Infof("Service has stopped")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Stopping Croupier client...")/c.logger.Infof("Stopping Croupier client...")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Client stopped successfully")/c.logger.Infof("Client stopped successfully")/g' pkg/croupier/client.go
sed -i '' 's/logInfof("Uploaded provider capabilities manifest")/c.logger.Infof("Uploaded provider capabilities manifest")/g' pkg/croupier/client.go

echo "Fixed logger calls in client.go"