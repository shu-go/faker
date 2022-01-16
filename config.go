package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"strings"

	"github.com/shu-go/clise"
)

// Config is saved as a JSON file.
type Config struct {
	// RootCommand (type *+Command) is a root node of Comand tree.
	RootCommand *Command `json:"cmds,omitempty"`
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

	content, err := ioutil.ReadAll(in)
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

// FindCommand takes commandline args and split into a Command and remaining args.
func (c Config) FindCommand(args []string) (*Command, []string, error) {
	curr := c.RootCommand
	//rog.Print(curr)
	lastIdx := -1
	for i, a := range args {
		//rog.Print(a)
		c, found := curr.SubCommands[a]
		//rog.Print(c, found)
		if !found {
			break
		}

		lastIdx = i
		curr = c
	}

	//rog.Print(lastIdx)
	if lastIdx == -1 {
		return nil, nil, errors.New("not found")
	}

	return curr, args[lastIdx+1:], nil
}

// AddCommand adds a Command newCmd to the location specified by names.
func (c *Config) AddCommand(names []string, newCmd Command) error {
	clise.Filter(&names, func(i int) bool {
		return strings.TrimSpace(names[i]) != ""
	})
	return c.RootCommand.AddSubCommand(names, newCmd)
}

// RemoveCommand removes a Command of the location specified by names.
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
