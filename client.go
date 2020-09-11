package vc

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/hashicorp/vault/api"
)

const (
	genericType  = "generic"
	mountRefresh = time.Minute
)

func isPermissionDenied(err error) bool {
	return strings.Contains(err.Error(), "* permission denied")
}

type completionFilter func(os.FileInfo) bool

func isAny(i os.FileInfo) bool {
	return true
}

func isDir(i os.FileInfo) bool {
	return i.IsDir()
}

func matchesFilters(i os.FileInfo, filters ...completionFilter) bool {
	for _, filter := range filters {
		if !filter(i) {
			return false
		}
	}
	return true
}

// Client for the Vault API
type Client struct {
	*api.Client

	// Path we are operating on, defaults to the root
	Path string

	// cachedMounts is a cached mounts lookup
	cachedMounts     map[string]*api.MountOutput
	cachedMountsTime time.Time
}

// NewClient builds a new Client
func NewClient(config *api.Config) (*Client, error) {
	var (
		c   = &Client{Path: "/"}
		err error
	)

	c.Client, err = api.NewClient(config)
	return c, err
}

// abspath resolves the absolute path
func (c *Client) abspath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(c.Path, path))
}

// mounts updates Client.cachedMounts if applicable
func (c *Client) mounts() (mounts map[string]*api.MountOutput, err error) {
	if time.Now().Add(-mountRefresh).After(c.cachedMountsTime) {
		mounts, err = c.Sys().ListMounts()
		if err == nil {
			c.cachedMounts = mounts
			c.cachedMountsTime = time.Now()
		}
	} else {
		mounts = c.cachedMounts
	}
	return
}

// Complete returns completer suggestions
func (c *Client) Complete(filters ...completionFilter) readline.DynamicCompleteFunc {
	return func(line string) []string {
		var path string
		if !strings.HasSuffix(line, " ") {
			// We are completing a partial match
			fields := strings.Fields(line)
			path = fields[len(fields)-1]
		}

		// Make absolute
		full := c.abspath(path)
		if strings.HasSuffix(path, "/") {
			full += "/*"
		} else {
			full += "*"
		}

		Debugf("complete %q -> %q in %q\n", path, full, c.Path)

		var suggestions []string
		if infos, err := c.Glob(full); err == nil {
			for _, info := range infos {
				Debugf("candidate %q\n", info.Name())
				if matchesFilters(info, filters...) {
					if info.IsDir() {
						suggestions = append(suggestions, info.Name()+"/")
					} else {
						suggestions = append(suggestions, info.Name())
					}
				}
			}
		}

		if !filepath.IsAbs(path) {
			if strings.HasPrefix(path, "..") {
				// We're completion a path relative to our current path with dotdot (..)
				// prefix, resolve relative path
				for i, name := range suggestions {
					if rel, _ := filepath.Rel(c.Path, name); rel == "." {
						// Special case: rel is our current directory
						suggestions[i] = "../" + filepath.Base(c.Path)
					} else {
						suggestions[i] = rel
					}
					Debugf("relative: %q -> %q", name, suggestions[i])
				}
			} else {
				// We're completing a path relative to our current path; strip our path
				// prefix from the suggestions
				for i, name := range suggestions {
					suggestions[i] = strings.TrimPrefix(name, c.Path+"/")
					Debugf("relative: %q -> %q", name, suggestions[i])
				}
			}
		}

		return suggestions
	}
}

// Stat mimicks an os.Stat call on Vault
func (c *Client) Stat(path string) (os.FileInfo, error) {
	// Make absolute
	path = c.abspath(path)
	Debugf("stat: %q", path)

	// Fast path for root
	if path == "/" {
		return &rootInfo{}, nil
	}

	// Check if the path is a file
	secret, err := c.Logical().Read(strings.TrimLeft(path, "/"))
	// Directories would get a permission denied error on Read(). So ignore it.
	if err != nil && !isPermissionDenied(err) {
		return nil, err
	}
	if secret != nil {
		return &secretInfo{
			Secret: secret,
			Path:   filepath.Clean(path),
			Key:    filepath.Base(path),
		}, nil
	}

	// Check if the path is a folder
	dir, _ := filepath.Split(path)
	if dir != "/" {
		// All folders in / are mounts, so skip this unless we're not in the root
		Debugf("stat: list %q", strings.TrimLeft(path, "/"))
		secret, err = c.Logical().List(strings.TrimLeft(path, "/"))
		if err != nil {
			return nil, err
		}
		if secret != nil {
			return &secretInfo{
				Secret: secret,
				Path:   strings.TrimRight(filepath.Clean(path), "/") + "/",
				Key:    strings.TrimRight(filepath.Clean(path), "/") + "/",
			}, nil
		}
	}

	// Finally check if our path is a mount
	mounts, err := c.mounts()
	if err != nil && !isPermissionDenied(err) {
		return nil, err
	}
	for name, mount := range mounts {
		name = "/" + name
		Debugf("stat: mount %q =~ %q?", name, path)
		if name == path {
			return &mountInfo{
				MountOutput: mount,
				Path:        name,
			}, nil
		} else if strings.HasPrefix(name, path+"/") {
			return &mountInfo{
				MountOutput: mount,
				Path:        path + "/",
			}, nil
		}
	}

	return nil, os.ErrNotExist
}

// ReadDir mimicks an ioutil.ReadDir call on Vault; permission errors are muted
func (c *Client) ReadDir(path string) ([]os.FileInfo, error) {
	Debugf("readdir: %q", strings.TrimLeft(path, "/"))
	var infos []os.FileInfo

	// Resolve path
	path = c.abspath(path)

	// Check mounts
	mounts, err := c.mounts()
	if err != nil && !isPermissionDenied(err) {
		return nil, err
	}
	for name, mount := range mounts {
		name = "/" + strings.TrimRight(name, "/")
		if mount.Type != genericType {
			continue
		}
		var (
			match bool
			base  = name
			dir   = filepath.Dir(name)
		)
		for len(dir) >= len(path) {
			if match = dir == path; match {
				break
			}
			base = dir
			dir = filepath.Dir(dir)
		}
		if match {
			infos = append(infos, &mountInfo{
				MountOutput: mount,
				Path:        base,
			})
		}
	}

	// Check secrets
	secret, err := c.Logical().List(strings.TrimLeft(path, "/"))
	if err != nil {
		return nil, err
	}
	if secret != nil {
		for _, key := range secret.Data["keys"].([]interface{}) {
			infos = append(infos, &secretInfo{
				Secret: secret,
				Path:   filepath.Clean(filepath.Join(strings.TrimRight(path, "/"), key.(string))),
				Key:    key.(string),
			})
		}
	}

	return infos, nil
}

func globExpression(pattern string) string {
	return strings.Replace(strings.Replace(pattern, "?", ".", -1), "*", ".*", -1)
}

func (c *Client) isGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?")
}

// Glob is a shortcut to list generic secrets and mounts by glob pattern. The
// wildcards "*" and "?" are supported. Currently only globbing the base of the
// path is supported, globbing on directory names is not.
func (c *Client) Glob(pattern string) ([]os.FileInfo, error) {
	Debugf("glob: %q", pattern)

	// Fast path, no globbing required
	if !c.isGlob(pattern) {
		info, err := c.Stat(pattern)
		return []os.FileInfo{info}, err
	}

	Debugf("glob abs: %q", c.abspath(pattern))
	dir, base := filepath.Split(c.abspath(pattern))
	if strings.ContainsAny(dir, "*?") {
		return nil, errors.New("directory globbing not supported")
	}
	if dir != "/" {
		dir = strings.TrimRight(dir, "/")
	}

	var (
		filter *regexp.Regexp
		err    error
	)
	if dir == "/" {
		filter, err = regexp.Compile(`^/` + globExpression(base))
	} else {
		filter, err = regexp.Compile(fmt.Sprintf("^%s/%s$", regexp.QuoteMeta(dir), globExpression(base)))
	}
	if err != nil {
		return nil, err
	}

	var infos []os.FileInfo
	items, err := c.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		Debugf("filter: %q =~ %s", item.Name(), filter)
		if filter.MatchString(item.Name()) {
			infos = append(infos, item)
		}
	}

	return infos, nil
}

// SetPath updates our working path
func (c *Client) SetPath(path string) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	c.Path = filepath.Clean(path)
}

// rootInfo mimick the root folder
type rootInfo struct{}

func (i *rootInfo) Name() string       { return "/" }
func (i *rootInfo) Size() int64        { return 0 }
func (i *rootInfo) Mode() os.FileMode  { return 0755 }
func (i *rootInfo) ModTime() time.Time { return time.Time{} }
func (i *rootInfo) IsDir() bool        { return true }
func (i *rootInfo) Sys() interface{}   { return nil }

// mountInfo is a wrapper for api.MountOutput that implements os.FileInfo
type mountInfo struct {
	*api.MountOutput
	Path string
}

func (i *mountInfo) Name() string       { return i.Path }
func (i *mountInfo) Size() int64        { return 0 }
func (i *mountInfo) Mode() os.FileMode  { return 0755 }
func (i *mountInfo) ModTime() time.Time { return time.Time{} }
func (i *mountInfo) IsDir() bool        { return true }
func (i *mountInfo) Sys() interface{}   { return i.MountOutput }

// secretInfo is a wrapper for api.Secret that implements os.FileInfo
type secretInfo struct {
	*api.Secret
	Path string
	Key  string
}

func (i *secretInfo) Name() string { return i.Path }
func (i *secretInfo) Size() int64  { return 0 }
func (i *secretInfo) Mode() os.FileMode {
	if i.IsDir() {
		return 0755
	}
	return 0644
}
func (i *secretInfo) ModTime() time.Time { return time.Time{} }
func (i *secretInfo) IsDir() bool        { return strings.HasSuffix(i.Key, "/") }
func (i *secretInfo) Sys() interface{}   { return i.Secret }
