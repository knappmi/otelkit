package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/knappmi/otelkit"
	"go.opentelemetry.io/otel/attribute"
)

// User represents a user in our system
type User struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	CreateAt time.Time `json:"created_at"`
}

// UserService handles user operations with in-memory storage
type UserService struct {
	users map[int]*User
	mutex sync.RWMutex
	kit   *otelkit.OTelKit
	nextID int
}

// NewUserService creates a new user service
func NewUserService(kit *otelkit.OTelKit) *UserService {
	service := &UserService{
		users:  make(map[int]*User),
		kit:    kit,
		nextID: 1,
	}
	
	// Add some sample data
	sampleUsers := []*User{
		{ID: 1, Name: "John Doe", Email: "john@example.com", CreateAt: time.Now()},
		{ID: 2, Name: "Jane Smith", Email: "jane@example.com", CreateAt: time.Now()},
		{ID: 3, Name: "Bob Johnson", Email: "bob@example.com", CreateAt: time.Now()},
	}
	
	for _, user := range sampleUsers {
		service.users[user.ID] = user
		if user.ID >= service.nextID {
			service.nextID = user.ID + 1
		}
	}
	
	return service
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, userID int) (*User, error) {
	ctx, span := s.kit.StartSpan(ctx, "user_service.get_user")
	defer span.End()
	
	s.kit.SetAttributes(ctx, attribute.Int("user.id", userID))

	// Simulate database operation
	err := s.kit.DatabaseOperation(ctx, "SELECT", "users", func(ctx context.Context) error {
		s.kit.SetAttributes(ctx, attribute.String("db.type", "memory"))

		s.kit.AddEvent(ctx, "query_start", attribute.String("query", "SELECT * FROM users WHERE id = ?"))

		// Simulate database access time
		time.Sleep(5 * time.Millisecond)

		s.mutex.RLock()
		_, exists := s.users[userID]
		s.mutex.RUnlock()

		if !exists {
			s.kit.AddEvent(ctx, "user_not_found")
			return fmt.Errorf("user not found")
		}

		s.kit.AddEvent(ctx, "user_found")
		return nil
	})

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	s.mutex.RLock()
	user := s.users[userID]
	s.mutex.RUnlock()

	return user, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
	ctx, span := s.kit.StartSpan(ctx, "user_service.create_user")
	defer span.End()
	
	s.kit.SetAttributes(ctx,
		attribute.String("user.name", name),
		attribute.String("user.email", email),
	)

	var newUser *User
	err := s.kit.DatabaseOperation(ctx, "INSERT", "users", func(ctx context.Context) error {
		s.kit.SetAttributes(ctx, attribute.String("db.type", "memory"))

		s.kit.AddEvent(ctx, "insert_start", attribute.String("query", "INSERT INTO users (name, email, created_at) VALUES (?, ?, ?)"))

		// Simulate database insert time
		time.Sleep(10 * time.Millisecond)

		s.mutex.Lock()
		userID := s.nextID
		s.nextID++
		
		newUser = &User{
			ID:       userID,
			Name:     name,
			Email:    email,
			CreateAt: time.Now(),
		}
		
		s.users[userID] = newUser
		s.mutex.Unlock()

		s.kit.AddEvent(ctx, "user_created", attribute.Int("user.id", userID))
		return nil
	})

	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return newUser, nil
}

// ProcessUserBatch processes multiple users
func (s *UserService) ProcessUserBatch(ctx context.Context, userIDs []int) error {
	return s.kit.BatchOperation(ctx, "process_users", len(userIDs), func(ctx context.Context) error {
		processed := 0
		failed := 0

		for _, userID := range userIDs {
			err := s.kit.TraceFunction(ctx, "process_single_user", func(ctx context.Context) error {
				s.kit.SetAttributes(ctx, attribute.Int("user.id", userID))

				user, err := s.GetUser(ctx, userID)
				if err != nil {
					failed++
					s.kit.AddEvent(ctx, "user_processing_failed")
					return err
				}

				// Simulate some processing
				time.Sleep(10 * time.Millisecond)
				s.kit.AddEvent(ctx, "user_processed",
					attribute.String("user.name", user.Name))

				processed++
				return nil
			})

			if err != nil {
				log.Printf("Failed to process user %d: %v", userID, err)
			}
		}

		s.kit.SetAttributes(ctx,
			attribute.Int("batch.processed", processed),
			attribute.Int("batch.failed", failed),
		)

		return nil
	})
}

func main() {
	// Initialize OTelKit
	config := otelkit.DefaultConfig()
	config.ServiceName = "user-api"
	config.ServiceVersion = "1.0.0"
	config.ExporterType = otelkit.ExporterJaeger
	config.Debug = true

	kit, err := otelkit.New(config)
	if err != nil {
		log.Fatalf("Failed to initialize OTelKit: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		kit.Shutdown(ctx)
	}()

	// Create user service (no database setup needed for in-memory)
	userService := NewUserService(kit)

	// Setup HTTP handlers
	setupHTTPHandlers(kit, userService)

	fmt.Println("Advanced example server starting on :8080")
	fmt.Println("Jaeger UI available at http://localhost:16686")
	fmt.Println()
	fmt.Println("Try these endpoints:")
	fmt.Println("  GET  /users/1          - Get user by ID")
	fmt.Println("  POST /users            - Create user (JSON body: {\"name\":\"...\", \"email\":\"...\"})")
	fmt.Println("  POST /users/batch      - Process user batch (JSON body: {\"user_ids\":[1,2,3]})")
	fmt.Println()

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupHTTPHandlers(kit *otelkit.OTelKit, userService *UserService) {
	mux := http.NewServeMux()

	// Get user endpoint
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		kit.SetAttributes(ctx, attribute.String("endpoint", "get_user"))

		// Extract user ID from path
		path := r.URL.Path
		userIDStr := path[len("/users/"):]
		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			kit.RecordError(ctx, fmt.Errorf("invalid user ID: %s", userIDStr))
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}

		user, err := userService.GetUser(ctx, userID)
		if err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	// Create user endpoint
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		kit.SetAttributes(ctx, attribute.String("endpoint", "create_user"))

		var request struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		user, err := userService.CreateUser(ctx, request.Name, request.Email)
		if err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	})

	// Batch processing endpoint
	mux.HandleFunc("/users/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		kit.SetAttributes(ctx, attribute.String("endpoint", "process_batch"))

		var request struct {
			UserIDs []int `json:"user_ids"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		err := userService.ProcessUserBatch(ctx, request.UserIDs)
		if err != nil {
			kit.RecordError(ctx, err)
			http.Error(w, "Batch processing failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "completed"})
	})

	// Wrap with OTel middleware
	handler := kit.HTTPMiddleware(mux)
	http.Handle("/", handler)
}
