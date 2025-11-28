/*
Package tmpl provides template processing for Releaser.
*/
package tmpl

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/oarkflow/releaser/internal/config"
	"github.com/oarkflow/releaser/internal/git"
)

// Context provides template context and rendering
type Context struct {
	config   *config.Config
	gitInfo  *git.Info
	snapshot bool
	nightly  bool
	data     map[string]interface{}
}

// New creates a new template context
func New(cfg *config.Config, gitInfo *git.Info, snapshot, nightly bool) *Context {
	ctx := &Context{
		config:   cfg,
		gitInfo:  gitInfo,
		snapshot: snapshot,
		nightly:  nightly,
		data:     make(map[string]interface{}),
	}
	ctx.init()
	return ctx
}

// init initializes the template data
func (c *Context) init() {
	now := time.Now()

	// Project info
	c.data["ProjectName"] = c.config.ProjectName

	// Version info
	if c.gitInfo != nil {
		c.data["Tag"] = c.gitInfo.CurrentTag
		c.data["PreviousTag"] = c.gitInfo.PreviousTag
		c.data["Version"] = strings.TrimPrefix(c.gitInfo.CurrentTag, "v")
		c.data["RawVersion"] = c.gitInfo.CurrentTag
		c.data["Major"] = c.gitInfo.Major
		c.data["Minor"] = c.gitInfo.Minor
		c.data["Patch"] = c.gitInfo.Patch
		c.data["Prerelease"] = c.gitInfo.PrereleaseSuffix
		c.data["IsPrerelease"] = c.gitInfo.Prerelease
		c.data["Metadata"] = c.gitInfo.Metadata
		c.data["Branch"] = c.gitInfo.Branch
		c.data["Commit"] = c.gitInfo.Commit
		c.data["ShortCommit"] = c.gitInfo.ShortCommit
		c.data["FullCommit"] = c.gitInfo.Commit
		c.data["CommitDate"] = c.gitInfo.CommitDate
		c.data["CommitTimestamp"] = c.gitInfo.CommitTimestamp
		c.data["GitURL"] = c.gitInfo.URL
		c.data["Summary"] = c.gitInfo.Summary
		c.data["TagSubject"] = c.gitInfo.TagSubject
		c.data["TagBody"] = c.gitInfo.TagBody
		c.data["TagContents"] = c.gitInfo.TagContents
	} else {
		c.data["Version"] = "0.0.0-SNAPSHOT"
		c.data["Tag"] = "v0.0.0-SNAPSHOT"
	}

	if c.snapshot {
		c.data["Version"] = "0.0.0-SNAPSHOT"
		c.data["IsSnapshot"] = true
	}

	if c.nightly {
		c.data["Version"] = now.Format("20060102")
		c.data["IsNightly"] = true
	}

	// Date/time
	c.data["Date"] = now.Format(time.RFC3339)
	c.data["Now"] = now
	c.data["Timestamp"] = now.Unix()

	// Runtime info
	c.data["Os"] = runtime.GOOS
	c.data["Arch"] = runtime.GOARCH
	c.data["Runtime"] = runtime.Version()

	// Environment
	env := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	c.data["Env"] = env

	// Custom variables from config
	for k, v := range c.config.Variables {
		c.data[k] = v
	}

	// Defaults from config
	c.data["Homepage"] = c.config.Defaults.Homepage
	c.data["Description"] = c.config.Defaults.Description
	c.data["License"] = c.config.Defaults.License
	c.data["Maintainer"] = c.config.Defaults.Maintainer
	c.data["Vendor"] = c.config.Defaults.Vendor
}

// Apply applies the template to a string
func (c *Context) Apply(tmpl string) (string, error) {
	t, err := template.New("").Funcs(c.funcs()).Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, c.data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Set sets a value in the context
func (c *Context) Set(key string, value interface{}) {
	c.data[key] = value
}

// Get gets a value from the context
func (c *Context) Get(key string) string {
	if val, ok := c.data[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetValue gets a raw value from the context
func (c *Context) GetValue(key string) interface{} {
	return c.data[key]
}

// WithArtifactInfo creates a context with artifact information (simple version)
func (c *Context) WithArtifactInfo(name, goos, goarch, goarm, goamd64 string) *Context {
	newCtx := &Context{
		config:   c.config,
		gitInfo:  c.gitInfo,
		snapshot: c.snapshot,
		nightly:  c.nightly,
		data:     make(map[string]interface{}),
	}

	// Copy existing data
	for k, v := range c.data {
		newCtx.data[k] = v
	}

	// Add artifact-specific data
	newCtx.data["ArtifactName"] = name
	newCtx.data["Os"] = goos
	newCtx.data["Arch"] = goarch
	newCtx.data["Arm"] = goarm
	newCtx.data["Amd64"] = goamd64

	// Add platform mappings
	newCtx.data["GOOS"] = goos
	newCtx.data["GOARCH"] = goarch
	newCtx.data["GOARM"] = goarm
	newCtx.data["GOAMD64"] = goamd64

	return newCtx
}

// WithArtifact is an alias for WithArtifactInfo for backward compatibility
func (c *Context) WithArtifact(name, goos, goarch, goarm, goamd64 string) *Context {
	return c.WithArtifactInfo(name, goos, goarch, goarm, goamd64)
}

// funcs returns the template function map
func (c *Context) funcs() template.FuncMap {
	return template.FuncMap{
		// String functions
		"replace":    strings.ReplaceAll,
		"tolower":    strings.ToLower,
		"toupper":    strings.ToUpper,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"trimprefix": strings.TrimPrefix,
		"trimsuffix": strings.TrimSuffix,
		"split":      strings.Split,
		"join":       strings.Join,
		"contains":   strings.Contains,
		"hasprefix":  strings.HasPrefix,
		"hassuffix":  strings.HasSuffix,
		"repeat":     strings.Repeat,
		"count":      strings.Count,
		"index":      strings.Index,
		"lastindex":  strings.LastIndex,
		"fields":     strings.Fields,

		// Environment
		"env":       os.Getenv,
		"expandenv": os.ExpandEnv,

		// Default value
		"default": func(def, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},

		// Conditional
		"if": func(cond bool, a, b interface{}) interface{} {
			if cond {
				return a
			}
			return b
		},

		// Date formatting
		"time": func(t time.Time, format string) string {
			return t.Format(format)
		},
		"now": time.Now,

		// Architecture mapping
		"archReplace": func(arch string, replacements ...string) string {
			if len(replacements)%2 != 0 {
				return arch
			}
			for i := 0; i < len(replacements); i += 2 {
				if arch == replacements[i] {
					return replacements[i+1]
				}
			}
			return arch
		},

		// OS mapping
		"osReplace": func(os string, replacements ...string) string {
			if len(replacements)%2 != 0 {
				return os
			}
			for i := 0; i < len(replacements); i += 2 {
				if os == replacements[i] {
					return replacements[i+1]
				}
			}
			return os
		},

		// Version helpers
		"incMajor": func() int {
			if c.gitInfo != nil {
				return c.gitInfo.Major + 1
			}
			return 1
		},
		"incMinor": func() int {
			if c.gitInfo != nil {
				return c.gitInfo.Minor + 1
			}
			return 1
		},
		"incPatch": func() int {
			if c.gitInfo != nil {
				return c.gitInfo.Patch + 1
			}
			return 1
		},

		// Markdown helpers
		"mdlink": func(text, url string) string {
			return "[" + text + "](" + url + ")"
		},
		"mdcode": func(code string) string {
			return "`" + code + "`"
		},
		"mdcodeblock": func(lang, code string) string {
			return "```" + lang + "\n" + code + "\n```"
		},

		// Filter helpers
		"filter": func(items []interface{}, key string, value interface{}) []interface{} {
			var result []interface{}
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					if m[key] == value {
						result = append(result, item)
					}
				}
			}
			return result
		},

		// List helpers
		"first": func(items []interface{}) interface{} {
			if len(items) > 0 {
				return items[0]
			}
			return nil
		},
		"last": func(items []interface{}) interface{} {
			if len(items) > 0 {
				return items[len(items)-1]
			}
			return nil
		},
		"reverse": func(items []interface{}) []interface{} {
			result := make([]interface{}, len(items))
			for i, item := range items {
				result[len(items)-1-i] = item
			}
			return result
		},
	}
}

// Data returns the template data
func (c *Context) Data() map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range c.data {
		result[k] = v
	}
	return result
}
