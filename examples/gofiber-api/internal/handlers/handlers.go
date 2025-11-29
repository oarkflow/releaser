// Package handlers provides HTTP request handlers for the API
package handlers

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/user/gofiber-api/internal/models"
	"github.com/user/gofiber-api/internal/store"
)

// Handler holds the dependencies for HTTP handlers
type Handler struct {
	store store.Store
}

// New creates a new Handler
func New(s store.Store) *Handler {
	return &Handler{store: s}
}

// ErrorHandler is the custom error handler for Fiber
func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(models.ErrorResponse{
		Code:    code,
		Message: message,
	})
}

// getPagination extracts pagination parameters from query
func getPagination(c *fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	return page, perPage
}

func calculateTotalPages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}

// User handlers

// ListUsers returns a paginated list of users
// @Summary List users
// @Description Get a paginated list of users
// @Tags users
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Router /users [get]
func (h *Handler) ListUsers(c *fiber.Ctx) error {
	page, perPage := getPagination(c)

	users, total, err := h.store.ListUsers(page, perPage)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(models.PaginatedResponse{
		Data:       users,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: calculateTotalPages(total, perPage),
	})
}

// GetUser returns a single user by ID
// @Summary Get user
// @Description Get a user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} models.User
// @Failure 404 {object} models.ErrorResponse
// @Router /users/{id} [get]
func (h *Handler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")

	user, err := h.store.GetUser(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(user)
}

// CreateUser creates a new user
// @Summary Create user
// @Description Create a new user
// @Tags users
// @Accept json
// @Produce json
// @Param user body models.CreateUserRequest true "User data"
// @Success 201 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /users [post]
func (h *Handler) CreateUser(c *fiber.Ctx) error {
	var req models.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Email == "" || req.Name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Email and name are required")
	}

	user, err := h.store.CreateUser(req)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return fiber.NewError(fiber.StatusConflict, "User with this email already exists")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(user)
}

// UpdateUser updates an existing user
// @Summary Update user
// @Description Update an existing user
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body models.UpdateUserRequest true "User data"
// @Success 200 {object} models.User
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /users/{id} [put]
func (h *Handler) UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")

	var req models.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	user, err := h.store.UpdateUser(id, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(user)
}

// DeleteUser deletes a user
// @Summary Delete user
// @Description Delete a user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 204 "No Content"
// @Failure 404 {object} models.ErrorResponse
// @Router /users/{id} [delete]
func (h *Handler) DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.DeleteUser(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "User not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Task handlers

// ListTasks returns a paginated list of tasks
// @Summary List tasks
// @Description Get a paginated list of tasks
// @Tags tasks
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Router /tasks [get]
func (h *Handler) ListTasks(c *fiber.Ctx) error {
	page, perPage := getPagination(c)

	tasks, total, err := h.store.ListTasks(page, perPage)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(models.PaginatedResponse{
		Data:       tasks,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: calculateTotalPages(total, perPage),
	})
}

// GetTask returns a single task by ID
// @Summary Get task
// @Description Get a task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.Task
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id} [get]
func (h *Handler) GetTask(c *fiber.Ctx) error {
	id := c.Params("id")

	task, err := h.store.GetTask(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Task not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(task)
}

// CreateTask creates a new task
// @Summary Create task
// @Description Create a new task
// @Tags tasks
// @Accept json
// @Produce json
// @Param task body models.CreateTaskRequest true "Task data"
// @Success 201 {object} models.Task
// @Failure 400 {object} models.ErrorResponse
// @Router /tasks [post]
func (h *Handler) CreateTask(c *fiber.Ctx) error {
	var req models.CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.Title == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Title is required")
	}

	if req.Priority < 1 {
		req.Priority = 3 // Default priority
	}

	task, err := h.store.CreateTask(req)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(task)
}

// UpdateTask updates an existing task
// @Summary Update task
// @Description Update an existing task
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param task body models.UpdateTaskRequest true "Task data"
// @Success 200 {object} models.Task
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id} [put]
func (h *Handler) UpdateTask(c *fiber.Ctx) error {
	id := c.Params("id")

	var req models.UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	task, err := h.store.UpdateTask(id, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Task not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(task)
}

// DeleteTask deletes a task
// @Summary Delete task
// @Description Delete a task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 204 "No Content"
// @Failure 404 {object} models.ErrorResponse
// @Router /tasks/{id} [delete]
func (h *Handler) DeleteTask(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.DeleteTask(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Task not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// Product handlers

// ListProducts returns a paginated list of products
// @Summary List products
// @Description Get a paginated list of products
// @Tags products
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param per_page query int false "Items per page" default(10)
// @Success 200 {object} models.PaginatedResponse
// @Router /products [get]
func (h *Handler) ListProducts(c *fiber.Ctx) error {
	page, perPage := getPagination(c)

	products, total, err := h.store.ListProducts(page, perPage)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(models.PaginatedResponse{
		Data:       products,
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: calculateTotalPages(total, perPage),
	})
}

// GetProduct returns a single product by ID
// @Summary Get product
// @Description Get a product by ID
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} models.Product
// @Failure 404 {object} models.ErrorResponse
// @Router /products/{id} [get]
func (h *Handler) GetProduct(c *fiber.Ctx) error {
	id := c.Params("id")

	product, err := h.store.GetProduct(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Product not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(product)
}

// CreateProduct creates a new product
// @Summary Create product
// @Description Create a new product
// @Tags products
// @Accept json
// @Produce json
// @Param product body models.CreateProductRequest true "Product data"
// @Success 201 {object} models.Product
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /products [post]
func (h *Handler) CreateProduct(c *fiber.Ctx) error {
	var req models.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	if req.SKU == "" || req.Name == "" || req.Price <= 0 || req.Currency == "" || req.Category == "" {
		return fiber.NewError(fiber.StatusBadRequest, "SKU, name, price, currency, and category are required")
	}

	product, err := h.store.CreateProduct(req)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return fiber.NewError(fiber.StatusConflict, "Product with this SKU already exists")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(product)
}

// UpdateProduct updates an existing product
// @Summary Update product
// @Description Update an existing product
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Param product body models.UpdateProductRequest true "Product data"
// @Success 200 {object} models.Product
// @Failure 400 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /products/{id} [put]
func (h *Handler) UpdateProduct(c *fiber.Ctx) error {
	id := c.Params("id")

	var req models.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	product, err := h.store.UpdateProduct(id, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Product not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(product)
}

// DeleteProduct deletes a product
// @Summary Delete product
// @Description Delete a product by ID
// @Tags products
// @Accept json
// @Produce json
// @Param id path string true "Product ID"
// @Success 204 "No Content"
// @Failure 404 {object} models.ErrorResponse
// @Router /products/{id} [delete]
func (h *Handler) DeleteProduct(c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.store.DeleteProduct(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return fiber.NewError(fiber.StatusNotFound, "Product not found")
		}
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.SendStatus(fiber.StatusNoContent)
}
