package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Command struct {
	Path string   `json:"path,omitempty"`
	Args []string `json:"args,omitempty"`

	Comment string `json:"comment,omitempty"`

	SubCommands map[string]*Command `json:"sub,omitempty"`
}

func (c Command) IsLeaf() bool {
	return c.Path != ""
}

func (c Command) PrintCommand(name string, byPath bool, indent int) {
	//rog.Printf("PrintCommand: %v, %v", name, indent)

	if c.IsLeaf() {
		spaces := strings.Repeat("  ", indent)
		fmt.Printf("\t%s%s:\t%s %v\n", spaces, name, c.Path, c.Args)
		return
	}

	keyNames := []struct {
		Key, Name string
	}{}
	for k := range c.SubCommands {
		key := k
		name := k
		if byPath {
			key = c.SubCommands[k].Path
		}
		keyNames = append(keyNames, struct{ Key, Name string }{Key: key, Name: name})
	}
	sort.Slice(keyNames, func(i, j int) bool {
		return keyNames[i].Key < keyNames[j].Key
	})

	for _, k := range keyNames {
		v := c.SubCommands[k.Name]
		spaces := strings.Repeat("  ", indent+1)
		//rog.Printf("PrintCommand: %v", k)
		if !v.IsLeaf() {
			fmt.Printf("\t%s%s:\n", spaces, k.Name)
		}
		v.PrintCommand(k.Name, byPath, indent+1)
	}
}

func (c *Command) AddSubCommand(names []string, newCmd Command) error {
	if len(names) == 0 {
		return nil
	}

	isLast := len(names) == 1
	name := names[0]
	//rog.Print("isLast: ", isLast)

	cmd, found := c.SubCommands[name]
	//rog.Print(found, cmd)

	if found {
		isLeaf := cmd.IsLeaf()

		if isLast {
			if isLeaf {
				*cmd = newCmd
			} else {
				return fmt.Errorf("remove the command %q first", name)
			}
		} else {
			if isLeaf {
				return fmt.Errorf("remove the command %q first", name)
			} else {
				return cmd.AddSubCommand(names[1:], newCmd)
			}
		}
	} else {
		if isLast {
			c.SubCommands[name] = &newCmd
		} else {
			c.SubCommands[name] = &Command{SubCommands: make(map[string]*Command)}
			return c.SubCommands[name].AddSubCommand(names[1:], newCmd)
			//rog.Print(curr)
		}
	}

	return nil
}

func (c *Command) RemoveSubCommand(names []string) error {
	if len(names) == 0 {
		return nil
	}

	isLast := len(names) == 1
	name := names[0]

	cmd, found := c.SubCommands[name]
	if found {
		isLeaf := cmd.IsLeaf()

		if isLast {
			delete(c.SubCommands, name)
		} else {
			if isLeaf {
				return errors.New("not found")
			} else {
				return cmd.RemoveSubCommand(names[1:])
			}
		}
	} else {
		return errors.New("not found")
	}

	return nil
}

func (c *Command) Clean() bool {
	subNames := []string{}
	for n, sub := range c.SubCommands {
		cleaned := sub.Clean()
		if cleaned {
			subNames = append(subNames, n)
		}
	}

	for _, n := range subNames {
		delete(c.SubCommands, n)
	}

	if len(c.SubCommands) == 0 {
		c.SubCommands = nil
		return !c.IsLeaf()
	}

	return false
}
