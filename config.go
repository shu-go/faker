package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/shu-go/clise"
)

// Config is saved as a JSON file.
type Config struct {
	// RootCommand (type *+Command) is a root node of Comand tree.
	RootCommand *Command `json:"cmds,omitempty"`

	SubMatch bool `json:"submatch,omitempty"`
}

// NewConfig returns a empty and working Config.
func NewConfig() *Config {
	return &Config{
		RootCommand: &Command{SubCommands: make(map[string]*Command)},
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
	if err != nil || c.RootCommand == nil {
		return nil, err
	}

	if c.RootCommand.SubCommands == nil {
		c.RootCommand.SubCommands = make(map[string]*Command)
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
	exact := !c.SubMatch
	cmd, args, err := c.RootCommand.FindCommand(args, exact)
	if err == nil && cmd == nil {
		return nil, nil, errors.New("not found")
	}
	return cmd, args, err
}

// AddCommand adds a Command newCmd to the location specified by names.
func (c *Config) AddCommand(names []string, newCmd Command) error {
	clise.Filter(&names, func(i int) bool {
		return strings.TrimSpace(names[i]) != ""
	})
	return c.RootCommand.AddSubCommand(names, newCmd)
}

// RemoveCommand removes a Command at the location specified by names.
func (c *Config) RemoveCommand(names []string) error {
	clise.Filter(&names, func(i int) bool {
		return strings.TrimSpace(names[i]) != ""
	})
	err := c.RootCommand.RemoveSubCommand(names)
	if err != nil {
		return err
	}

	c.RootCommand.Clean()

	return nil
}
