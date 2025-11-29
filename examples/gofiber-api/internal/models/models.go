// Package models defines data structures for the API
package models

import "time"

// User represents a user in the system
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest represents the request body for creating a user
type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Role  string `json:"role" validate:"omitempty,oneof=admin user guest"`
}

// UpdateUserRequest represents the request body for updating a user
type UpdateUserRequest struct {
	Email  *string `json:"email,omitempty" validate:"omitempty,email"`
	Name   *string `json:"name,omitempty" validate:"omitempty,min=2,max=100"`
	Role   *string `json:"role,omitempty" validate:"omitempty,oneof=admin user guest"`
	Active *bool   `json:"active,omitempty"`
}

// Task represents a task/todo item
type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	UserID      string    `json:"user_id,omitempty"`
	DueDate     time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateTaskRequest represents the request body for creating a task
type CreateTaskRequest struct {
	Title       string `json:"title" validate:"required,min=1,max=200"`
	Description string `json:"description" validate:"max=1000"`
	Priority    int    `json:"priority" validate:"min=1,max=5"`
	UserID      string `json:"user_id,omitempty"`
	DueDate     string `json:"due_date,omitempty"`
}

// UpdateTaskRequest represents the request body for updating a task
type UpdateTaskRequest struct {
	Title       *string `json:"title,omitempty" validate:"omitempty,min=1,max=200"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	Status      *string `json:"status,omitempty" validate:"omitempty,oneof=pending in_progress completed cancelled"`
	Priority    *int    `json:"priority,omitempty" validate:"omitempty,min=1,max=5"`
	DueDate     *string `json:"due_date,omitempty"`
}

// Product represents a product in the catalog
type Product struct {
	ID          string    `json:"id"`
	SKU         string    `json:"sku"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Currency    string    `json:"currency"`
	Stock       int       `json:"stock"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateProductRequest represents the request body for creating a product
type CreateProductRequest struct {
	SKU         string   `json:"sku" validate:"required,min=3,max=50"`
	Name        string   `json:"name" validate:"required,min=1,max=200"`
	Description string   `json:"description" validate:"max=2000"`
	Price       float64  `json:"price" validate:"required,gt=0"`
	Currency    string   `json:"currency" validate:"required,len=3"`
	Stock       int      `json:"stock" validate:"min=0"`
	Category    string   `json:"category" validate:"required"`
	Tags        []string `json:"tags,omitempty"`
}

// UpdateProductRequest represents the request body for updating a product
type UpdateProductRequest struct {
	Name        *string   `json:"name,omitempty" validate:"omitempty,min=1,max=200"`
	Description *string   `json:"description,omitempty" validate:"omitempty,max=2000"`
	Price       *float64  `json:"price,omitempty" validate:"omitempty,gt=0"`
	Stock       *int      `json:"stock,omitempty" validate:"omitempty,min=0"`
	Category    *string   `json:"category,omitempty"`
	Tags        *[]string `json:"tags,omitempty"`
	Active      *bool     `json:"active,omitempty"`
}

// PaginatedResponse wraps paginated results
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PerPage    int         `json:"per_page"`
	Total      int         `json:"total"`
	TotalPages int         `json:"total_pages"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}
