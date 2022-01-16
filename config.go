package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/shu-go/clise"
)

type Config struct {
	RootCommand *Command `json:"cmds,omitempty"`
}

func NewConfig() *Config {
	return &Config{
		RootCommand: &Command{SubCommands: make(map[string]*Command)},
	}
}

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

func (c Config) PrintCommands(configPath string, byPath bool) {
	fmt.Println("Commands:")
	if c.RootCommand != nil {
		c.RootCommand.PrintCommand("", byPath, -1)
	}

	fmt.Println("")
	fmt.Printf("Config: %s\n", configPath)
}

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

func (c *Config) AddCommand(names []string, newCmd Command) error {
	clise.Filter(&names, func(i int) bool {
		return strings.TrimSpace(names[i]) != ""
	})
	return c.RootCommand.AddSubCommand(names, newCmd)
}

/*
func (c *Config) AddCommand(names []string, newCmd Command) error {
	curr := c.RootCommand
	for i, n := range names {
		//rog.Print("*", i, n)

		isLast := i == len(names)-1
		//rog.Print("isLast: ", isLast)

		//rog.Print(curr)
		cmd, found := curr.SubCommands[n]
		//rog.Print(found, cmd)

		if found {
			isLeaf := cmd.IsLeaf()

			if isLast {
				if isLeaf {
					*cmd = newCmd
				} else {
					return fmt.Errorf("remove the command %q first", n)
				}
			} else {
				if isLeaf {
					return fmt.Errorf("remove the command %q first", n)
				} else {
					curr = cmd
				}
			}
		} else {
			if isLast {
				curr.SubCommands[n] = &newCmd
			} else {
				curr.SubCommands[n] = &Command{SubCommands: make(map[string]*Command)}
				curr = curr.SubCommands[n]
				//rog.Print(curr)
			}
		}
	}

	return nil
}
*/

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

/*
func (c *Config) RemoveCommand(names []string) error {
	curr := c.RootCommand
	for i, n := range names {
		//rog.Print("*", i, n)

		isLast := i == len(names)-1
		//rog.Print("isLast: ", isLast)

		//rog.Print(curr)
		cmd, found := curr.SubCommands[n]
		//rog.Print(found, cmd)

		if found {
			isLeaf := cmd.IsLeaf()

			if isLast {
				delete(curr.SubCommands, n)
			} else {
				if isLeaf {
					return errors.New("not found")
				} else {
					curr = cmd
				}
			}
		} else {
			return errors.New("not found")
		}
	}

	c.RootCommand.Clean()

	return nil
}
*/
