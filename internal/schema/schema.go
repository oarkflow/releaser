// Package schema provides JSON schema validation for releaser configuration.
package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/log"
	"gopkg.in/yaml.v3"
)

// Schema represents a JSON Schema for validation
type Schema struct {
	ID          string             `json:"$id,omitempty"`
	Schema      string             `json:"$schema,omitempty"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Type        string             `json:"type,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []interface{}      `json:"enum,omitempty"`
	Default     interface{}        `json:"default,omitempty"`
	Minimum     *float64           `json:"minimum,omitempty"`
	Maximum     *float64           `json:"maximum,omitempty"`
	MinLength   *int               `json:"minLength,omitempty"`
	MaxLength   *int               `json:"maxLength,omitempty"`
	Pattern     string             `json:"pattern,omitempty"`
	Format      string             `json:"format,omitempty"`
	OneOf       []*Schema          `json:"oneOf,omitempty"`
	AnyOf       []*Schema          `json:"anyOf,omitempty"`
	AllOf       []*Schema          `json:"allOf,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
	Defs        map[string]*Schema `json:"$defs,omitempty"`
}

// ValidationError represents a schema validation error
type ValidationError struct {
	Path    string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// ValidationResult contains all validation errors
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// Validator validates YAML/JSON against schemas
type Validator struct {
	schema *Schema
	defs   map[string]*Schema
}

// NewValidator creates a new validator with the given schema
func NewValidator(schema *Schema) *Validator {
	defs := make(map[string]*Schema)
	if schema.Defs != nil {
		for k, v := range schema.Defs {
			defs["#/$defs/"+k] = v
		}
	}
	return &Validator{
		schema: schema,
		defs:   defs,
	}
}

// ValidateFile validates a YAML or JSON file
func (v *Validator) ValidateFile(path string) *ValidationResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return &ValidationResult{
			Valid: false,
			Errors: []ValidationError{{
				Path:    path,
				Message: fmt.Sprintf("failed to read file: %v", err),
			}},
		}
	}

	var doc interface{}
	if strings.HasSuffix(path, ".json") {
		if err := json.Unmarshal(data, &doc); err != nil {
			return &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Path:    path,
					Message: fmt.Sprintf("invalid JSON: %v", err),
				}},
			}
		}
	} else {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Path:    path,
					Message: fmt.Sprintf("invalid YAML: %v", err),
				}},
			}
		}
	}

	return v.Validate(doc)
}

// Validate validates a document against the schema
func (v *Validator) Validate(doc interface{}) *ValidationResult {
	result := &ValidationResult{Valid: true}
	v.validate(v.schema, doc, "", result)
	result.Valid = len(result.Errors) == 0
	return result
}

// validate recursively validates a value against a schema
func (v *Validator) validate(schema *Schema, value interface{}, path string, result *ValidationResult) {
	if schema == nil {
		return
	}

	// Handle $ref
	if schema.Ref != "" {
		if refSchema, ok := v.defs[schema.Ref]; ok {
			v.validate(refSchema, value, path, result)
		}
		return
	}

	// Handle oneOf, anyOf, allOf
	if len(schema.OneOf) > 0 {
		matches := 0
		for _, s := range schema.OneOf {
			subResult := &ValidationResult{Valid: true}
			v.validate(s, value, path, subResult)
			if subResult.Valid {
				matches++
			}
		}
		if matches != 1 {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("must match exactly one of the schemas (matched %d)", matches),
				Value:   value,
			})
		}
		return
	}

	if len(schema.AnyOf) > 0 {
		matched := false
		for _, s := range schema.AnyOf {
			subResult := &ValidationResult{Valid: true}
			v.validate(s, value, path, subResult)
			if subResult.Valid {
				matched = true
				break
			}
		}
		if !matched {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: "must match at least one schema",
				Value:   value,
			})
		}
		return
	}

	if len(schema.AllOf) > 0 {
		for _, s := range schema.AllOf {
			v.validate(s, value, path, result)
		}
		return
	}

	// Type validation
	if schema.Type != "" {
		if !v.checkType(schema.Type, value) {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("expected type %s, got %T", schema.Type, value),
				Value:   value,
			})
			return
		}
	}

	// Enum validation
	if len(schema.Enum) > 0 {
		found := false
		for _, e := range schema.Enum {
			if value == e {
				found = true
				break
			}
		}
		if !found {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value must be one of: %v", schema.Enum),
				Value:   value,
			})
		}
	}

	// String validations
	if str, ok := value.(string); ok {
		if schema.MinLength != nil && len(str) < *schema.MinLength {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string length must be at least %d", *schema.MinLength),
				Value:   value,
			})
		}
		if schema.MaxLength != nil && len(str) > *schema.MaxLength {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("string length must be at most %d", *schema.MaxLength),
				Value:   value,
			})
		}
	}

	// Number validations
	if num, ok := toFloat(value); ok {
		if schema.Minimum != nil && num < *schema.Minimum {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value must be at least %v", *schema.Minimum),
				Value:   value,
			})
		}
		if schema.Maximum != nil && num > *schema.Maximum {
			result.Errors = append(result.Errors, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("value must be at most %v", *schema.Maximum),
				Value:   value,
			})
		}
	}

	// Object validation
	if obj, ok := value.(map[string]interface{}); ok {
		// Check required fields
		for _, req := range schema.Required {
			if _, exists := obj[req]; !exists {
				result.Errors = append(result.Errors, ValidationError{
					Path:    joinPath(path, req),
					Message: "required field is missing",
				})
			}
		}

		// Validate properties
		if schema.Properties != nil {
			for key, propSchema := range schema.Properties {
				if propValue, exists := obj[key]; exists {
					v.validate(propSchema, propValue, joinPath(path, key), result)
				}
			}
		}
	}

	// Array validation
	if arr, ok := value.([]interface{}); ok {
		if schema.Items != nil {
			for i, item := range arr {
				v.validate(schema.Items, item, fmt.Sprintf("%s[%d]", path, i), result)
			}
		}
	}
}

// checkType checks if a value matches the expected type
func (v *Validator) checkType(expected string, value interface{}) bool {
	if value == nil {
		return expected == "null"
	}

	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := toFloat(value)
		return ok
	case "integer":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		case float32, float64:
			f, _ := toFloat(value)
			return f == float64(int64(f))
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "null":
		return value == nil
	}
	return false
}

// toFloat converts a value to float64
func toFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// joinPath joins path segments
func joinPath(base, key string) string {
	if base == "" {
		return key
	}
	return base + "." + key
}

// GenerateSchema generates a JSON Schema for the releaser configuration
func GenerateSchema() *Schema {
	return &Schema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		ID:          "https://releaser.dev/schema/v1",
		Title:       "Releaser Configuration",
		Description: "Schema for releaser.yaml configuration file",
		Type:        "object",
		Properties: map[string]*Schema{
			"project_name": {
				Type:        "string",
				Description: "The name of the project",
			},
			"version": {
				Type:        "string",
				Description: "The version of the project",
			},
			"env": {
				Type:        "array",
				Description: "Environment variables",
				Items: &Schema{
					Type: "string",
				},
			},
			"before": {
				Ref: "#/$defs/hooks",
			},
			"after": {
				Ref: "#/$defs/hooks",
			},
			"builds": {
				Type:        "array",
				Description: "Build configurations",
				Items: &Schema{
					Ref: "#/$defs/build",
				},
			},
			"archives": {
				Type:        "array",
				Description: "Archive configurations",
				Items: &Schema{
					Ref: "#/$defs/archive",
				},
			},
			"nfpms": {
				Type:        "array",
				Description: "Linux package configurations",
				Items: &Schema{
					Ref: "#/$defs/nfpm",
				},
			},
			"checksum": {
				Ref: "#/$defs/checksum",
			},
			"changelog": {
				Ref: "#/$defs/changelog",
			},
			"release": {
				Ref: "#/$defs/release",
			},
			"dockers": {
				Type:        "array",
				Description: "Docker image configurations",
				Items: &Schema{
					Ref: "#/$defs/docker",
				},
			},
			"brews": {
				Type:        "array",
				Description: "Homebrew tap configurations",
				Items: &Schema{
					Ref: "#/$defs/brew",
				},
			},
			"signs": {
				Type:        "array",
				Description: "Signing configurations",
				Items: &Schema{
					Ref: "#/$defs/sign",
				},
			},
			"sboms": {
				Type:        "array",
				Description: "SBOM configurations",
				Items: &Schema{
					Ref: "#/$defs/sbom",
				},
			},
			"announce": {
				Ref: "#/$defs/announce",
			},
		},
		Defs: map[string]*Schema{
			"hooks": {
				Type: "object",
				Properties: map[string]*Schema{
					"hooks": {
						Type: "array",
						Items: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"cmd": {Type: "string"},
								"env": {
									Type:  "array",
									Items: &Schema{Type: "string"},
								},
								"dir": {Type: "string"},
							},
							Required: []string{"cmd"},
						},
					},
				},
			},
			"build": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":      {Type: "string"},
					"main":    {Type: "string"},
					"binary":  {Type: "string"},
					"dir":     {Type: "string"},
					"builder": {Type: "string"},
					"goos": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"goarch": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"goarm": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"ldflags": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"flags": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"env": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"install": {
						Type: "array",
						Items: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"type":      {Type: "string"},
								"cmd":       {Type: "string"},
								"env":       {Type: "object"},
								"dir":       {Type: "string"},
								"if":        {Type: "string"},
								"fail_fast": {Type: "boolean"},
								"shell":     {Type: "boolean"},
								"output":    {Type: "string"},
							},
						},
					},
					"ignore": {
						Type: "array",
						Items: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"goos":   {Type: "string"},
								"goarch": {Type: "string"},
							},
						},
					},
				},
			},
			"archive": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":               {Type: "string"},
					"name_template":    {Type: "string"},
					"format":           {Type: "string", Enum: []interface{}{"tar.gz", "tgz", "tar.xz", "txz", "tar.zst", "tar", "gz", "zip", "binary"}},
					"format_overrides": {Type: "array"},
					"files": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"builds": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
				},
			},
			"nfpm": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":           {Type: "string"},
					"package_name": {Type: "string"},
					"vendor":       {Type: "string"},
					"homepage":     {Type: "string"},
					"maintainer":   {Type: "string"},
					"description":  {Type: "string"},
					"license":      {Type: "string"},
					"formats": {
						Type:  "array",
						Items: &Schema{Type: "string", Enum: []interface{}{"deb", "rpm", "apk", "archlinux"}},
					},
					"dependencies": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"recommends": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"suggests": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"conflicts": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"section":  {Type: "string"},
					"priority": {Type: "string"},
				},
			},
			"checksum": {
				Type: "object",
				Properties: map[string]*Schema{
					"name_template": {Type: "string"},
					"algorithm":     {Type: "string", Enum: []interface{}{"sha256", "sha512", "sha1", "md5", "sha384", "sha224"}},
					"extra_files": {
						Type:  "array",
						Items: &Schema{Type: "object"},
					},
				},
			},
			"changelog": {
				Type: "object",
				Properties: map[string]*Schema{
					"sort":    {Type: "string", Enum: []interface{}{"asc", "desc", ""}},
					"use":     {Type: "string"},
					"abbrev":  {Type: "integer"},
					"filters": {Type: "object"},
					"groups": {
						Type: "array",
						Items: &Schema{
							Type: "object",
							Properties: map[string]*Schema{
								"title":  {Type: "string"},
								"regexp": {Type: "string"},
								"order":  {Type: "integer"},
							},
						},
					},
				},
			},
			"release": {
				Type: "object",
				Properties: map[string]*Schema{
					"github": {
						Type: "object",
						Properties: map[string]*Schema{
							"owner": {Type: "string"},
							"name":  {Type: "string"},
						},
					},
					"gitlab": {
						Type: "object",
						Properties: map[string]*Schema{
							"owner": {Type: "string"},
							"name":  {Type: "string"},
						},
					},
					"gitea": {
						Type: "object",
						Properties: map[string]*Schema{
							"owner": {Type: "string"},
							"name":  {Type: "string"},
						},
					},
					"draft":         {Type: "boolean"},
					"prerelease":    {Type: "string"},
					"make_latest":   {Type: "string"},
					"name_template": {Type: "string"},
					"disable":       {Type: "boolean"},
					"skip_upload":   {Type: "boolean"},
					"extra_files":   {Type: "array"},
					"header":        {Type: "string"},
					"footer":        {Type: "string"},
				},
			},
			"docker": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":         {Type: "string"},
					"goos":       {Type: "string"},
					"goarch":     {Type: "string"},
					"goarm":      {Type: "string"},
					"dockerfile": {Type: "string"},
					"image_templates": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"build_flag_templates": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"extra_files": {
						Type:  "array",
						Items: &Schema{Type: "string"},
					},
					"use":       {Type: "string"},
					"skip_push": {Type: "string"},
				},
			},
			"brew": {
				Type: "object",
				Properties: map[string]*Schema{
					"name":        {Type: "string"},
					"description": {Type: "string"},
					"homepage":    {Type: "string"},
					"license":     {Type: "string"},
					"repository": {
						Type: "object",
						Properties: map[string]*Schema{
							"owner":  {Type: "string"},
							"name":   {Type: "string"},
							"branch": {Type: "string"},
							"token":  {Type: "string"},
						},
					},
					"folder":      {Type: "string"},
					"install":     {Type: "string"},
					"test":        {Type: "string"},
					"caveats":     {Type: "string"},
					"skip_upload": {Type: "string"},
				},
			},
			"sign": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":        {Type: "string"},
					"cmd":       {Type: "string"},
					"args":      {Type: "array", Items: &Schema{Type: "string"}},
					"artifacts": {Type: "string"},
					"signature": {Type: "string"},
					"output":    {Type: "boolean"},
				},
			},
			"sbom": {
				Type: "object",
				Properties: map[string]*Schema{
					"id":        {Type: "string"},
					"cmd":       {Type: "string"},
					"args":      {Type: "array", Items: &Schema{Type: "string"}},
					"artifacts": {Type: "string"},
					"documents": {Type: "array", Items: &Schema{Type: "string"}},
				},
			},
			"announce": {
				Type: "object",
				Properties: map[string]*Schema{
					"twitter":  {Type: "object"},
					"slack":    {Type: "object"},
					"discord":  {Type: "object"},
					"telegram": {Type: "object"},
					"webhook":  {Type: "object"},
				},
			},
		},
	}
}

// ValidateConfig validates a releaser configuration file
func ValidateConfig(path string) *ValidationResult {
	schema := GenerateSchema()
	validator := NewValidator(schema)
	result := validator.ValidateFile(path)

	if !result.Valid {
		log.Error("Configuration validation failed", "errors", len(result.Errors))
		for _, err := range result.Errors {
			log.Error("Validation error", "path", err.Path, "message", err.Message)
		}
	}

	return result
}

// WriteSchema writes the schema to a file
func WriteSchema(path string) error {
	schema := GenerateSchema()
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
