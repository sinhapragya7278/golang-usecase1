package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Record struct {
	CID   string `json:"cid"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

var db *sql.DB

// Load environment variables from .env file
func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found. Using system environment variables.")
	} else {
		log.Println("Environment variables loaded successfully from .env file.")
	}
}

// Initialize the database connection with retry mechanism
func initDB() {
	var err error
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", ""),
		getEnv("DB_NAME", "postgres"),
	)

	for i := 0; i < 5; i++ { // Retry up to 5 times
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			if pingErr := db.Ping(); pingErr == nil {
				log.Println("Database connection established.")
				break
			} else {
				log.Printf("Database ping failed (attempt %d/5): %v. Retrying in 2 seconds...", i+1, pingErr)
			}
		} else {
			log.Printf("Database connection failed (attempt %d/5): %v. Retrying in 2 seconds...", i+1, err)
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Unable to connect to the database after retries: %v", err)
	}

	// Ensure table exists
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS records (
            id SERIAL PRIMARY KEY,
            cid TEXT UNIQUE, 
            name TEXT NOT NULL, 
            image TEXT
        )`)
	if err != nil {
		log.Fatalf("Error creating table: %v", err)
	}
	log.Println("Database table initialized successfully.")
}

// Load CSV data and insert it into the database
func loadCSVAndInsertData(filePath string) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Printf("CSV file not found: %s. Skipping data insertion.", filePath)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatalf("Unable to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		log.Fatalf("Unable to read CSV file: %v", err)
	}

	for i, record := range records {
		if len(record) < 3 { // Ensure all required fields are present
			log.Printf("Skipping invalid record at line %d: %v", i+1, record)
			continue
		}

		_, err := db.Exec(`
            INSERT INTO records (cid, name, image) 
            VALUES ($1, $2, $3) ON CONFLICT (cid) DO NOTHING`,
			record[0], record[1], record[2])
		if err != nil {
			log.Printf("Error inserting record (line %d): %v", i+1, err)
		}
	}
	log.Println("CSV data inserted into the database successfully.")
}

// Handle API requests to fetch data
func fetchDataHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT cid, name, image FROM records`)
	if err != nil {
		log.Printf("Error fetching records: %v", err)
		http.Error(w, "Unable to fetch records", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.CID, &record.Name, &record.Image); err != nil {
			log.Printf("Error scanning row: %v", err)
			http.Error(w, "Error reading data", http.StatusInternalServerError)
			return
		}
		records = append(records, record)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(records); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
	log.Println("Data fetched and returned successfully.")
}

// Helper function to get environment variables with a fallback
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func main() {
	loadEnv()
	initDB()
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	loadCSVAndInsertData("data.csv")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/data", http.StatusPermanentRedirect)
	})
	http.HandleFunc("/data", fetchDataHandler)
	log.Println("Server started on port 8080")
	if err := http.ListenAndServe("0.0.0.0:8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
