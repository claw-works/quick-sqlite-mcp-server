package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/claw-works/quick-sqlite-mcp-server/internal/db"
	"github.com/claw-works/quick-sqlite-mcp-server/internal/mcp"
	"github.com/claw-works/quick-sqlite-mcp-server/internal/tools"
)

func main() {
	// Initialize database manager
	rootDir := os.Getenv("ROOT_DIR")
	if rootDir == "" {
		rootDir = "/data"
	}
	allowAbsolute := os.Getenv("ALLOW_ABSOLUTE_PATH") == "true"

	dbManager := db.NewManager(rootDir, allowAbsolute)
	defer dbManager.Close()

	// Initialize tool registry
	registry := tools.NewRegistry(dbManager)

	// MCP server runs on stdio
	server := mcp.NewServer(registry)

	log.SetOutput(os.Stderr)
	log.Printf("quick-sqlite-mcp-server started (ROOT_DIR=%s, ALLOW_ABSOLUTE_PATH=%v)", rootDir, allowAbsolute)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024) // 10MB buffer
	writer := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		response := server.Handle(line)
		if response != nil {
			if err := writer.Encode(response); err != nil {
				fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin error: %v", err)
	}
}
