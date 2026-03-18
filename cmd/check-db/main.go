package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "test-data/croupier.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Check all tables
	fmt.Println("=== TABLES ===")
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- %s\n", name)
		count++
	}
	if count == 0 {
		fmt.Println("NO TABLES FOUND!")
	}

	// Check admins
	fmt.Println("\n=== ADMINS ===")
	rows2, err2 := db.Query("SELECT id, username, nickname, status FROM admins")
	if err2 != nil {
		fmt.Printf("Error querying admins: %v\n", err2)
		return
	}
	defer rows2.Close()

	fmt.Println("ID | Username | Nickname | Status")
	fmt.Println("---|----------|----------|-------")
	adminCount := 0
	for rows2.Next() {
		var id int
		var username, nickname string
		var status int
		if err := rows2.Scan(&id, &username, &nickname, &status); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d | %s | %s | %d\n", id, username, nickname, status)
		adminCount++
	}
	if adminCount == 0 {
		fmt.Println("NO ADMINS FOUND!")
	}

	// Check admin_roles
	fmt.Println("\n=== ADMIN_ROLES ===")
	rows3, err3 := db.Query("SELECT admin_id, role_id FROM admin_roles")
	if err3 != nil {
		fmt.Printf("Error querying admin_roles: %v\n", err3)
		return
	}
	defer rows3.Close()

	fmt.Println("AdminID | RoleID")
	fmt.Println("--------|-------")
	roleCount := 0
	for rows3.Next() {
		var adminID, roleID int
		if err := rows3.Scan(&adminID, &roleID); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d | %d\n", adminID, roleID)
		roleCount++
	}
	if roleCount == 0 {
		fmt.Println("NO ADMIN_ROLES FOUND!")
	}

	// Check roles
	fmt.Println("\n=== ROLES ===")
	rows4, err4 := db.Query("SELECT id, name FROM roles LIMIT 10")
	if err4 != nil {
		fmt.Printf("Error querying roles: %v\n", err4)
		return
	}
	defer rows4.Close()

	fmt.Println("ID | Name")
	fmt.Println("---|----")
	for rows4.Next() {
		var id int
		var name string
		if err := rows4.Scan(&id, &name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d | %s\n", id, name)
	}
}
