package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/shu-go/orderedmap"
	yaml "gopkg.in/yaml.v3"
)

// Config is saved as a JSON file.
type Config struct {
	Version int `json:"version" yaml:"version"`

	// RootCommand (type *+Command) is a root node of Comand tree.
	Commands *orderedmap.OrderedMap[string, Command] `json:"commands,omitempty" yaml:"commands,omitempty"`

	SubMatch bool `json:"submatch,omitempty" yaml:"submatch,omitempty"`
}

// NewConfig returns a empty and working Config.
func NewConfig() *Config {
	return &Config{
		Version: 2,
	}
}

// LoadConfig read from a Reader in (JSON formatted content required).
func LoadConfig(in io.Reader) (*Config, error) {
	var c Config

	content, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(content, &c)
	if err != nil {
		return nil, err
	}

	if c.Commands == nil {
		c.Commands = orderedmap.New[string, Command]()
	}

	return &c, nil
}

func LoadYAMLConfig(in io.Reader) (*Config, error) {
	var c Config

	content, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(content, &c)
	if err != nil {
		return nil, err
	}

	if c.Commands == nil {
		c.Commands = orderedmap.New[string, Command]()
	}

	return &c, nil
}

// Save writes to a Writer out.
func (c Config) Save(out io.Writer) error {
	content, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	_, err = out.Write(content)
	if err != nil {
		return err
	}

	return nil
}

func (c Config) SaveYAML(out io.Writer) error {
	content, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	_, err = out.Write(content)
	if err != nil {
		return err
	}

	return nil
}

func (c Config) PrintVariables(out io.Writer) {
	fmt.Fprintf(out, "\tsubmatch: %v\n", c.SubMatch)
}

func (c *Config) SetVariables(args []string) error {
	for i := 0; i < len(args)/2; i++ {
		switch args[i] {
		case "submatch":
			test, ok := strconv.ParseBool(args[i+1])
			if ok != nil {
				return fmt.Errorf("value %q is invalid for config entry %q", args[i+1], args[i])
			}
			c.SubMatch = test

		default:
			return fmt.Errorf("config entry %q not found", args[i])
		}
	}

	return nil
}

// FindCommand takes commandline args and split into a Command and remaining args.
func (c Config) FindCommand(args []string) (*Command, []string, error) {
	keys := c.Commands.Keys()
	var names [][]string

	for _, k := range keys {
		names = append(names, strings.Split(k, "."))
	}
	slices.SortFunc(names, func(a, b []string) int {
		lena := len(a)
		lenb := len(b)
		l := lena
		if l > lenb {
			l = lenb
		}

		for i := 0; i < l; i++ {
			cmp := strings.Compare(a[i], b[i])
			if cmp != 0 {
				return cmp
			}
			if cmp == 0 && i == l-1 /*last*/ {
				if lena < lenb {
					return -1
				}
			}
		}
		return 1
	})

	// challenge submatch

	filtered := slices.Clone(names)
	if c.SubMatch {
		filtered = slices.DeleteFunc(filtered, deleteBy(args, strings.HasPrefix))
	}
	slices.SortFunc(filtered, sortByLen)

	if len(filtered) > 1 && len(filtered[0]) == len(filtered[1]) {
		// challenge exact match

		filtered2 := slices.Clone(filtered)
		filtered2 = slices.DeleteFunc(filtered2, deleteBy(args, strings.EqualFold))

		if len(filtered2) == 0 && len(filtered) > 1 && len(filtered[0]) == len(filtered[1]) {
			return nil, nil, fmt.Errorf("Ambiguous. %+v?", filtered)
		}

		filtered = filtered2

		slices.SortFunc(filtered, sortByLen)
	}

	if len(filtered) > 0 {
		slices.SortFunc(filtered, sortByLen)

		key := strings.Join(filtered[0], ".")
		cmd, _ := c.Commands.Get(key)
		ss := strings.Split(key, ".")
		return &cmd, args[len(ss):], nil
	}

	return nil, nil, errors.New("Not found")
}

// AddCommand adds a Command newCmd to the location specified by names.
func (c *Config) AddCommand(names []string, newCmd Command) {
	c.Commands.Set(strings.Join(names, "."), newCmd)
}

// RemoveCommand removes a Command at the location specified by names.
func (c *Config) RemoveCommand(names []string) {
	c.Commands.Delete(strings.Join(names, "."))
}

// PrintCommand prints Commands to stddout.
func (c *Config) PrintCommand(byPath bool) {
	type command struct {
		Name string
		Path string
		Args []string
	}

	keys := c.Commands.Keys()
	slices.Sort(keys)

	cmds := make([]command, 0, len(keys))
	for _, k := range keys {
		cmd, _ := c.Commands.Get(k)

		cmds = append(cmds, command{
			Name: k,
			Path: cmd.Path,
			Args: cmd.Args,
		})
	}
	slices.SortFunc(cmds, func(a, b command) int {
		if byPath {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Name, b.Name)
	})

	for _, cmd := range cmds {
		fmt.Printf("\t%s:\t%s %v\n", cmd.Name, cmd.Path, cmd.Args)
	}
}

func (c *Config) Upgrade(configPath string) error {
	type OldCommand0 struct {
		Name string
		Path string
		Args []string
	}
	type OldConfig0 struct {
		Commands []OldCommand0
	}
	//
	type OldCommand1 struct {
		Path        string                  `json:"path,omitempty" yaml:"path,omitempty"`
		Args        []string                `json:"args,omitempty" yaml:"args,omitempty,flow"`
		SubCommands map[string]*OldCommand1 `json:"sub,omitempty" yaml:"sub,omitempty"`
	}
	type OldConfig1 struct {
		RootCommand *OldCommand1 `json:"cmds,omitempty" yaml:"cmds,omitempty"`
		SubMatch    bool         `json:"submatch,omitempty" yaml:"submatch,omitempty"`
	}

	//

	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "backup config file to %q\n", configPath+".bak")

	os.Remove(configPath + ".bak")
	os.Rename(configPath, configPath+".bak")

	fmt.Fprintln(os.Stderr, "version 1 -> 2")

	var old1 OldConfig1
	if in(filepath.Ext(configPath), ".yaml", ".yml") {
		err = yaml.Unmarshal(content, &old1)
	} else {
		err = json.Unmarshal(content, &old1)
	}

	if err == nil {
		c.Version = 2
		c.SubMatch = old1.SubMatch
		c.Commands = orderedmap.New[string, Command]()

		var up func(string, map[string]*OldCommand1) error
		up = func(prefix string, cmds map[string]*OldCommand1) error {
			for k, v := range cmds {
				cmd := Command{
					Path: v.Path,
					Args: v.Args,
				}
				name := k
				if prefix != "" {
					name = prefix + "." + k
				}

				c.Commands.Set(name, cmd)

				if err := up(k, v.SubCommands); err != nil {
					return err
				}
			}
			return nil
		}
		err = up("", old1.RootCommand.SubCommands)
		if err != nil {
			return err
		}

		configPath = configPath[:len(configPath)-len(filepath.Ext(configPath))] + ".yaml"
		content, err := yaml.Marshal(c)
		if err != nil {
			return err
		}

		return os.WriteFile(configPath, content, os.ModePerm)
	}

	fmt.Fprintln(os.Stderr, "version 0 -> 2")

	var old0 OldConfig0
	err = json.Unmarshal(content, &old0)
	if err != nil {
		return err
	}

	return nil
}

// desc
func sortByLen(a, b []string) int {
	if len(a) < len(b) {
		return 1
	}
	if len(a) > len(b) {
		return -1
	}
	return 0
}

func deleteBy(args []string, match func(a, b string) bool) func([]string) bool {
	return func(ss []string) bool {
		last := -1
		for ia, a := range args {
			if len(ss) < ia+1 {
				// not all names in f given by args
				break
			}
			if !match(ss[ia], a) {
				// name mismatch
				break
			}
			last = ia
		}
		return len(ss) != last+1
	}
}
