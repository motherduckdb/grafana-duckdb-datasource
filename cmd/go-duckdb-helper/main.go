package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	_ "github.com/marcboeker/go-duckdb"
)

func main() {
	var (
		sqlCmd     = flag.String("s", "", "SQL command to execute (multiple commands can be separated by semicolons)")
		dbPath     = flag.String("db", "", "Path to DuckDB database file (empty for in-memory)")
	)

	flag.Parse()

	if *sqlCmd == "" {
		log.Fatal("SQL command is required")
	}

	// Prepare database path
	dsn := "?access_mode=read_write"
	if *dbPath != "" {
		absPath, err := filepath.Abs(*dbPath)
		if err != nil {
			log.Fatalf("Failed to get absolute path: %v", err)
		}
		dsn = absPath + dsn
	}

	fmt.Printf("Using DSN: %s\n", dsn)

	// Open database connection
	db, err := sql.Open("duckdb", dsn)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Split the input command into individual commands
	commands := strings.Split(*sqlCmd, ";")

	// Execute each command
	for _, cmd := range commands {
		// Skip empty commands
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		fmt.Printf("Executing: %s\n", cmd)
		result, err := db.Exec(cmd)
		if err != nil {
			log.Fatalf("Failed to execute SQL '%s': %v", cmd, err)
		}
		if result != nil {
			rows, _ := result.RowsAffected()
			fmt.Printf("Rows affected: %d\n", rows)
		}
	}

	fmt.Println("All commands executed successfully")
} 
