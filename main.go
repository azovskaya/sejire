package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
)

const dbFile = "database.json"

// ─── Data Structures ───────────────────────────────────────────────────────

type Person struct {
	ID         string `json:"ID"`
	TargetID   string `json:"TargetID"`
	FullName   string `json:"FullName"`
	Sex        string `json:"Sex"`
	BirthDate  string `json:"BirthDate"`
	DeathDate  string `json:"DeathDate"`
	BirthPlace string `json:"BirthPlace"`
	DeathPlace string `json:"DeathPlace"` // Место захоронения
	Role       string `json:"Role"`
}

type Commit struct {
	Hash      string `json:"Hash"`
	Timestamp string `json:"Timestamp"`
	Action    string `json:"Action"`
	PersonID  string `json:"PersonID"`
	Details   string `json:"Details"`
}

type Database struct {
	Persons     []Person `json:"persons"`
	Commits     []Commit `json:"commits"`
	ArweaveHash string   `json:"arweaveHash"`
	LastSaved   string   `json:"lastSaved"`
}

// ─── DB Helpers ─────────────────────────────────────────────────────────────

func loadDB() Database {
	data, err := os.ReadFile(dbFile)
	if err != nil {
		return Database{Persons: []Person{}, Commits: []Commit{}, ArweaveHash: "", LastSaved: ""}
	}
	var db Database
	if err := json.Unmarshal(data, &db); err != nil {
		return Database{Persons: []Person{}, Commits: []Commit{}}
	}
	if db.Persons == nil {
		db.Persons = []Person{}
	}
	if db.Commits == nil {
		db.Commits = []Commit{}
	}
	return db
}

func saveDB(db Database) error {
	db.LastSaved = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dbFile, data, 0o644)
}

// ─── Utilities ───────────────────────────────────────────────────────────────

func formatName(s string) string {
	words := strings.Fields(strings.TrimSpace(s))
	for i, w := range words {
		runes := []rune(w)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func generateID() string {
	return fmt.Sprintf("INDI_%d%04d", time.Now().Unix(), rand.Intn(10000))
}

func makeCommit(action, personID, details string) Commit {
	payload := action + "|" + personID + "|" + details + "|" + time.Now().String()
	h := sha256.Sum256([]byte(payload))
	return Commit{
		Hash:      fmt.Sprintf("%x", h),
		Timestamp: time.Now().Format(time.RFC3339),
		Action:    action,
		PersonID:  personID,
		Details:   details,
	}
}

func ValidateConnection(members []Person, targetID string, role string) error {
	for _, p := range members {
		if p.TargetID == targetID && p.Role == role {
			return fmt.Errorf("ошибка: у этого потомка уже есть %s", role)
		}
	}
	return nil
}

// ─── HTTP Middleware ─────────────────────────────────────────────────────────

func noCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// ─── Handlers ────────────────────────────────────────────────────────────────

func handlePersons(w http.ResponseWriter, r *http.Request) {
	cors(w)
	noCache(w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.Method {
	case http.MethodGet:
		db := loadDB()
		if err := json.NewEncoder(w).Encode(db); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	case http.MethodPost:
		var p Person
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		db := loadDB()
		if strings.TrimSpace(p.FullName) == "" {
			http.Error(w, "FullName is required", http.StatusBadRequest)
			return
		}
		if err := ValidateConnection(db.Persons, p.TargetID, p.Role); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p.ID = generateID()
		p.FullName = formatName(p.FullName)
		db.Persons = append(db.Persons, p)
		commit := makeCommit("ADD", p.ID, p.FullName+" ("+p.Role+")")
		db.Commits = append(db.Commits, commit)
		if err := saveDB(db); err != nil {
			http.Error(w, "save error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp := map[string]interface{}{"person": p, "commit": commit}
		json.NewEncoder(w).Encode(resp)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDatabase(w http.ResponseWriter, r *http.Request) {
	cors(w)
	noCache(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := os.Remove(dbFile); err != nil && !os.IsNotExist(err) {
		http.Error(w, "remove error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	cors(w)
	noCache(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var db Database
	if err := json.NewDecoder(r.Body).Decode(&db); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	if db.Persons == nil {
		db.Persons = []Person{}
	}
	if db.Commits == nil {
		db.Commits = []Commit{}
	}
	if err := saveDB(db); err != nil {
		http.Error(w, "save error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "imported"})
}

func handleSnapshot(w http.ResponseWriter, r *http.Request) {
	cors(w)
	noCache(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	db := loadDB()
	data, _ := json.Marshal(db.Persons)
	h := sha256.Sum256(data)
	txID := fmt.Sprintf("ar_%x", h[:16])
	db.ArweaveHash = txID
	commit := makeCommit("SNAPSHOT", "system", "Arweave TX: "+txID)
	db.Commits = append(db.Commits, commit)
	if err := saveDB(db); err != nil {
		http.Error(w, "save error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"arweaveHash": txID,
		"explorerURL": "https://viewblock.io/arweave/tx/" + txID,
		"status":      "snapshot_created",
		"commit":      commit,
	})
}

func handlePersonByID(w http.ResponseWriter, r *http.Request) {
	cors(w)
	noCache(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/persons/")
	if id == "" {
		http.Error(w, "missing ID", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		handleDeletePersonByID(w, r, id)
	case http.MethodPut:
		handleUpdatePersonByID(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleDeletePersonByID(w http.ResponseWriter, r *http.Request, id string) {
	db := loadDB()
	newList := make([]Person, 0, len(db.Persons))
	found := false
	for _, p := range db.Persons {
		if p.ID == id {
			found = true
			continue
		}
		newList = append(newList, p)
	}
	if !found {
		http.Error(w, "person not found", http.StatusNotFound)
		return
	}
	db.Persons = newList
	commit := makeCommit("DELETE", id, "Person removed")
	db.Commits = append(db.Commits, commit)
	saveDB(db)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func handleUpdatePersonByID(w http.ResponseWriter, r *http.Request, id string) {
	var p Person
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	p.ID = id
	p.FullName = formatName(p.FullName)
	db := loadDB()
	idx := -1
	for i, x := range db.Persons {
		if x.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		http.Error(w, "person not found", http.StatusNotFound)
		return
	}
	db.Persons[idx] = p
	commit := makeCommit("UPDATE", id, p.FullName+" ("+p.Role+")")
	db.Commits = append(db.Commits, commit)
	saveDB(db)
	json.NewEncoder(w).Encode(map[string]interface{}{"person": p, "commit": commit})
}

// ─── Router & Main ───────────────────────────────────────────────────────────

func main() {
	rand.Seed(time.Now().UnixNano())

	mux := http.NewServeMux()

	mux.HandleFunc("/api/persons", handlePersons)
	mux.HandleFunc("/api/persons/", handlePersonByID)
	mux.HandleFunc("/api/database", handleDatabase)
	mux.HandleFunc("/api/import", handleImport)
	mux.HandleFunc("/api/snapshot", handleSnapshot)

	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)

	// Получаем порт от Render
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Слушаем на 0.0.0.0 для внешнего доступа
	addr := "0.0.0.0:" + port
	log.Printf("🌳 SEJIRE server running on %s\n", addr)
	
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
