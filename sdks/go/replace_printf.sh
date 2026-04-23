#!/bin/bash

# Script to replace fmt.Printf with logger calls in Go SDK

# Replace in client.go
sed -i.bak 's/fmt\.Printf("Registered function: %s (version: %s)\\n"/logInfof("Registered function: %s (version: %s")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("🔌 Connecting to Croupier Agent: %s\\n"/logInfof("Connecting to Croupier Agent: %s")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("✅ Successfully connected and registered with Agent\\n"/logInfof("Successfully connected and registered with Agent")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("📍 Local service address: %s\\n"/logInfof("Local service address: %s")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("🔑 Session ID: %s\\n"/logInfof("Session ID: %s")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("⚠️ Failed to register capabilities: %v\\n"/logWarnf("Failed to register capabilities: %v")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("🚀 Croupier client service started\\n"/logInfof("Croupier client service started")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("🎯 Registered functions: %d\\n"/logInfof("Registered functions: %d")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("💡 Use Stop\(\) to stop the service\\n"/logInfof("Use Stop() to stop the service")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Printf("===============================================\\n"/logInfof("===============================================")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("🛑 Service stopped by Stop\(\) call")/logInfof("Service stopped by Stop() call")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("🛑 Service stopped by context cancellation")/logInfof("Service stopped by context cancellation")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("🛑 Service has stopped")/logInfof("Service has stopped")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("🛑 Stopping Croupier client...")/logInfof("Stopping Croupier client...")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("✅ Client stopped successfully")/logInfof("Client stopped successfully")/' pkg/croupier/client.go
sed -i.bak 's/fmt\.Println("📤 Uploaded provider capabilities manifest")/logInfof("Uploaded provider capabilities manifest")/' pkg/croupier/client.go

# Replace in invoker.go
sed -i.bak 's/fmt\.Printf("Connecting to server\/agent at: %s\\n"/logInfof("Connecting to server/agent at: %s")/' pkg/croupier/invoker.go
sed -i.bak 's/fmt\.Printf("Connected to: %s\\n"/logInfof("Connected to: %s")/' pkg/croupier/invoker.go
sed -i.bak 's/fmt\.Printf("Set schema for function: %s\\n"/logInfof("Set schema for function: %s")/' pkg/croupier/invoker.go
sed -i.bak 's/fmt\.Println("Invoker closed")/logInfof("Invoker closed")/' pkg/croupier/invoker.go

# Replace in grpc_manager.go
sed -i.bak 's/fmt\.Printf("✅ Successfully connected to Agent: %s\\n"/logInfof("Successfully connected to Agent: %s")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Println("📴 Disconnected from Agent")/logInfof("Disconnected from Agent")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("📡 Registered service with Agent\\n"/logInfof("Registered service with Agent")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("   Service ID: %s\\n"/logInfof("   Service ID: %s")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("   Version: %s\\n"/logInfof("   Version: %s")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("   Local Address: %s\\n"/logInfof("   Local Address: %s")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("   Functions: %d\\n"/logInfof("   Functions: %d")/' pkg/croupier/grpc_manager.go
sed -i.bak 's/fmt\.Printf("🚀 Local gRPC server started on: %s\\n"/logInfof("Local gRPC server started on: %s")/' pkg/croupier/grpc_manager.go

# Clean up backup files
rm -f pkg/croupier/*.bak

echo "Done replacing fmt.Printf with logger calls"