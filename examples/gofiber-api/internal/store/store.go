// Package store provides data storage implementations
package store

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/user/gofiber-api/internal/models"
)

// Common errors
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
)

// Store defines the interface for data storage
type Store interface {
	// Users
	ListUsers(page, perPage int) ([]models.User, int, error)
	GetUser(id string) (*models.User, error)
	CreateUser(req models.CreateUserRequest) (*models.User, error)
	UpdateUser(id string, req models.UpdateUserRequest) (*models.User, error)
	DeleteUser(id string) error

	// Tasks
	ListTasks(page, perPage int) ([]models.Task, int, error)
	GetTask(id string) (*models.Task, error)
	CreateTask(req models.CreateTaskRequest) (*models.Task, error)
	UpdateTask(id string, req models.UpdateTaskRequest) (*models.Task, error)
	DeleteTask(id string) error

	// Products
	ListProducts(page, perPage int) ([]models.Product, int, error)
	GetProduct(id string) (*models.Product, error)
	CreateProduct(req models.CreateProductRequest) (*models.Product, error)
	UpdateProduct(id string, req models.UpdateProductRequest) (*models.Product, error)
	DeleteProduct(id string) error
}

// MemoryStore implements Store with in-memory storage
type MemoryStore struct {
	mu       sync.RWMutex
	users    map[string]*models.User
	tasks    map[string]*models.Task
	products map[string]*models.Product
}

// NewMemoryStore creates a new in-memory store with sample data
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		users:    make(map[string]*models.User),
		tasks:    make(map[string]*models.Task),
		products: make(map[string]*models.Product),
	}

	// Add sample data
	s.seedData()
	return s
}

func (s *MemoryStore) seedData() {
	now := time.Now()

	// Sample users
	users := []models.User{
		{ID: uuid.New().String(), Email: "admin@example.com", Name: "Admin User", Role: "admin", Active: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Email: "john@example.com", Name: "John Doe", Role: "user", Active: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Email: "jane@example.com", Name: "Jane Smith", Role: "user", Active: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range users {
		s.users[users[i].ID] = &users[i]
	}

	// Sample tasks
	tasks := []models.Task{
		{ID: uuid.New().String(), Title: "Setup development environment", Description: "Install all required tools", Status: "completed", Priority: 1, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Title: "Implement user authentication", Description: "Add JWT-based auth", Status: "in_progress", Priority: 2, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), Title: "Write unit tests", Description: "Add tests for handlers", Status: "pending", Priority: 3, CreatedAt: now, UpdatedAt: now},
	}
	for i := range tasks {
		s.tasks[tasks[i].ID] = &tasks[i]
	}

	// Sample products
	products := []models.Product{
		{ID: uuid.New().String(), SKU: "LAPTOP-001", Name: "Pro Laptop", Description: "High-performance laptop", Price: 1299.99, Currency: "USD", Stock: 50, Category: "Electronics", Tags: []string{"laptop", "computer"}, Active: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), SKU: "PHONE-001", Name: "Smart Phone", Description: "Latest smartphone", Price: 799.99, Currency: "USD", Stock: 100, Category: "Electronics", Tags: []string{"phone", "mobile"}, Active: true, CreatedAt: now, UpdatedAt: now},
		{ID: uuid.New().String(), SKU: "BOOK-001", Name: "Go Programming", Description: "Learn Go programming language", Price: 49.99, Currency: "USD", Stock: 200, Category: "Books", Tags: []string{"book", "programming", "go"}, Active: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range products {
		s.products[products[i].ID] = &products[i]
	}
}

// User operations

func (s *MemoryStore) ListUsers(page, perPage int) ([]models.User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]models.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, *u)
	}

	total := len(users)
	start := (page - 1) * perPage
	if start >= total {
		return []models.User{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}

	return users[start:end], total, nil
}

func (s *MemoryStore) GetUser(id string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return user, nil
}

func (s *MemoryStore) CreateUser(req models.CreateUserRequest) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate email
	for _, u := range s.users {
		if u.Email == req.Email {
			return nil, ErrAlreadyExists
		}
	}

	now := time.Now()
	role := req.Role
	if role == "" {
		role = "user"
	}

	user := &models.User{
		ID:        uuid.New().String(),
		Email:     req.Email,
		Name:      req.Name,
		Role:      role,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.users[user.ID] = user
	return user, nil
}

func (s *MemoryStore) UpdateUser(id string, req models.UpdateUserRequest) (*models.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[id]
	if !ok {
		return nil, ErrNotFound
	}

	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Name != nil {
		user.Name = *req.Name
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.Active != nil {
		user.Active = *req.Active
	}
	user.UpdatedAt = time.Now()

	return user, nil
}

func (s *MemoryStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	return nil
}

// Task operations

func (s *MemoryStore) ListTasks(page, perPage int) ([]models.Task, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]models.Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		tasks = append(tasks, *t)
	}

	total := len(tasks)
	start := (page - 1) * perPage
	if start >= total {
		return []models.Task{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}

	return tasks[start:end], total, nil
}

func (s *MemoryStore) GetTask(id string) (*models.Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return task, nil
}

func (s *MemoryStore) CreateTask(req models.CreateTaskRequest) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	task := &models.Task{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Status:      "pending",
		Priority:    req.Priority,
		UserID:      req.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.DueDate != "" {
		if due, err := time.Parse(time.RFC3339, req.DueDate); err == nil {
			task.DueDate = due
		}
	}

	s.tasks[task.ID] = task
	return task, nil
}

func (s *MemoryStore) UpdateTask(id string, req models.UpdateTaskRequest) (*models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[id]
	if !ok {
		return nil, ErrNotFound
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.DueDate != nil {
		if due, err := time.Parse(time.RFC3339, *req.DueDate); err == nil {
			task.DueDate = due
		}
	}
	task.UpdatedAt = time.Now()

	return task, nil
}

func (s *MemoryStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}

// Product operations

func (s *MemoryStore) ListProducts(page, perPage int) ([]models.Product, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	products := make([]models.Product, 0, len(s.products))
	for _, p := range s.products {
		products = append(products, *p)
	}

	total := len(products)
	start := (page - 1) * perPage
	if start >= total {
		return []models.Product{}, total, nil
	}
	end := start + perPage
	if end > total {
		end = total
	}

	return products[start:end], total, nil
}

func (s *MemoryStore) GetProduct(id string) (*models.Product, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	product, ok := s.products[id]
	if !ok {
		return nil, ErrNotFound
	}
	return product, nil
}

func (s *MemoryStore) CreateProduct(req models.CreateProductRequest) (*models.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for duplicate SKU
	for _, p := range s.products {
		if p.SKU == req.SKU {
			return nil, ErrAlreadyExists
		}
	}

	now := time.Now()
	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	product := &models.Product{
		ID:          uuid.New().String(),
		SKU:         req.SKU,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Currency:    req.Currency,
		Stock:       req.Stock,
		Category:    req.Category,
		Tags:        tags,
		Active:      true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.products[product.ID] = product
	return product, nil
}

func (s *MemoryStore) UpdateProduct(id string, req models.UpdateProductRequest) (*models.Product, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	product, ok := s.products[id]
	if !ok {
		return nil, ErrNotFound
	}

	if req.Name != nil {
		product.Name = *req.Name
	}
	if req.Description != nil {
		product.Description = *req.Description
	}
	if req.Price != nil {
		product.Price = *req.Price
	}
	if req.Stock != nil {
		product.Stock = *req.Stock
	}
	if req.Category != nil {
		product.Category = *req.Category
	}
	if req.Tags != nil {
		product.Tags = *req.Tags
	}
	if req.Active != nil {
		product.Active = *req.Active
	}
	product.UpdatedAt = time.Now()

	return product, nil
}

func (s *MemoryStore) DeleteProduct(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.products[id]; !ok {
		return ErrNotFound
	}
	delete(s.products, id)
	return nil
}
