package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/shu-go/orderedmap"
	yaml "gopkg.in/yaml.v3"
)

var (
	ErrNotFound = errors.New("command not found")
	ErrLocked   = errors.New("locked")
)

// Config is saved as a JSON file.
type Config struct {
	Version int `json:"version" yaml:"version"`

	// RootCommand (type *+Command) is a root node of Comand tree.
	Commands *orderedmap.OrderedMap[string, Command] `json:"commands,omitempty" yaml:"commands,omitempty"`

	SubMatch bool `json:"submatch,omitempty" yaml:"submatch,omitempty"`
	AutoLock bool `json:"autolock,omitempty" yaml:"autolock,omitempty"`
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
	fmt.Fprintf(out, "\tautlock: %v\n", c.AutoLock)
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

		case "autolock":
			test, ok := strconv.ParseBool(args[i+1])
			if ok != nil {
				return fmt.Errorf("value %q is invalid for config entry %q", args[i+1], args[i])
			}
			c.AutoLock = test

		default:
			return fmt.Errorf("config entry %q not found", args[i])
		}
	}

	return nil
}

// FindCommand takes commandline args and split into a Command and remaining args.
func (c Config) FindCommand(args []string) (*Command, []string, error) {
	keys := c.Commands.Keys()
	names := make([][]string, 0, len(keys))
	for _, k := range keys {
		names = append(names, strings.Split(k, "."))
	}

	slices.SortFunc(names, sortBySliceElems)

	rr := ranked(names)

	// challenge submatch

	if c.SubMatch {
		rr.rank(strings.HasPrefix, args)

		if len(rr) == 0 {
			return nil, nil, ErrNotFound
		}

		if !rr.hasMultipleTopRankers() {
			cmd, args := c.getCmdAndArgs(rr[0], args)
			return cmd, args, nil
		}

		exactRR := slices.Clone(rr)
		exactRR.rank(strings.EqualFold, args)

		if len(exactRR) == 0 {
			return nil, nil, fmt.Errorf("Ambiguous. %s?", rr.topRankers().allNames())
		} else if len(exactRR) > 1 {
			return nil, nil, fmt.Errorf("Ambiguous. %s?", exactRR.topRankers().allNames())
		}

		cmd, args := c.getCmdAndArgs(exactRR[0], args)
		return cmd, args, nil
	}

	rr.rank(strings.EqualFold, args)

	if len(rr) == 0 {
		return nil, nil, ErrNotFound
	}

	if !rr.hasMultipleTopRankers() {
		cmd, args := c.getCmdAndArgs(rr[0], args)
		return cmd, args, nil
	}

	return nil, nil, ErrNotFound
}

// AddCommand adds a Command newCmd to the location specified by names.
func (c *Config) AddCommand(names []string, newCmd Command) error {
	key := strings.Join(names, ".")
	cmd, found := c.Commands.Get(key)
	if found && cmd.Locked {
		return ErrLocked
	}

	newCmd.Locked = newCmd.Locked || c.AutoLock

	if c.Commands == nil {
		c.Commands = orderedmap.New[string, Command]()
	}

	c.Commands.Set(key, newCmd)
	return nil
}

// RemoveCommand removes a Command at the location specified by names.
func (c *Config) RemoveCommand(names []string) error {
	key := strings.Join(names, ".")
	cmd, found := c.Commands.Get(key)
	if found && cmd.Locked {
		return ErrLocked
	}

	c.Commands.Delete(key)
	return nil
}

func (c *Config) LockCommand(names []string, locked bool) {
	key := strings.Join(names, ".")
	cmd, found := c.Commands.Get(key)
	if found {
		cmd.Locked = locked
		c.Commands.Set(key, cmd)
	}
}

// PrintCommand prints Commands to stddout.
func (c *Config) PrintCommand(byPath bool) {
	type command struct {
		Name   string
		Path   string
		Args   []string
		Locked bool
	}

	keys := c.Commands.Keys()
	slices.Sort(keys)

	cmds := make([]command, 0, len(keys))
	for _, k := range keys {
		cmd, _ := c.Commands.Get(k)

		cmds = append(cmds, command{
			Name:   k,
			Path:   cmd.Path,
			Args:   cmd.Args,
			Locked: cmd.Locked,
		})
	}
	slices.SortFunc(cmds, func(a, b command) int {
		if byPath {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.Name, b.Name)
	})

	bold := color.New(color.FgHiYellow, color.Bold)
	for _, cmd := range cmds {
		if cmd.Locked {
			fmt.Print("\t")
			bold.Print(cmd.Name)
			fmt.Printf(":\t%s %v", cmd.Path, cmd.Args)
			bold.Print(" +LOCKED+")
			fmt.Print("\n")
		} else {
			fmt.Printf("\t%s:\t%s %v\n", cmd.Name, cmd.Path, cmd.Args)
		}
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

	bakfile := configPath + time.Now().Format("20060102150405") + ".bak"
	os.Remove(bakfile)
	os.Rename(configPath, bakfile)

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

// sort by slice[0] asc, slice[1] asc, ...
//
// x y
// x y z
// x z
// y y
// y y y
// y y z
// z x x
func sortBySliceElems(a, b []string) int {
	lena := len(a)
	lenb := len(b)

	minlen := lena
	if minlen > lenb {
		minlen = lenb
	}

	for i := 0; i < minlen; i++ {
		cmp := strings.Compare(a[i], b[i])
		if cmp != 0 {
			return cmp
		}
		if cmp == 0 && i == minlen-1 /*last*/ {
			if lena < lenb {
				return -1
			}
		}
	}
	return 1
}

type rankedName struct {
	rank int
	name []string
}

type rankedNames []rankedName

func ranked(names [][]string) rankedNames {
	count := len(names)
	rr := make(rankedNames, 0, count)
	for i := 0; i < count; i++ {
		rr = append(rr, rankedName{
			rank: 0,
			name: names[i],
		})
	}
	return rr
}

func (c Config) getCmdAndArgs(r rankedName, args []string) (*Command, []string) {
	key := strings.Join(r.name, ".")
	cmd, found := c.Commands.Get(key)
	if !found {
		return nil, nil
	}
	return &cmd, args[len(r.name):]
}

func (rr *rankedNames) rank(match func(a, b string) bool, args []string) rankedNames {
	ss := make(rankedNames, 0, len(*rr))

	for ri := 0; ri < len(*rr); ri++ {
		if len((*rr)[ri].name) > len(args) {
			(*rr)[ri].rank = 0
			continue
		}

		ok := true
		for ni := 0; ni < len((*rr)[ri].name); ni++ {
			if !match((*rr)[ri].name[ni], args[ni]) {
				ok = false
				break
			}
		}
		if ok {
			(*rr)[ri].rank = len((*rr)[ri].name)
		} else {
			(*rr)[ri].rank = 0
		}
	}

	(*rr) = slices.DeleteFunc((*rr), func(r rankedName) bool {
		return r.rank == 0
	})

	// desc
	slices.SortStableFunc((*rr), func(a, b rankedName) int {
		return -cmp.Compare(a.rank, b.rank)
	})

	return ss
}

func (rr rankedNames) hasMultipleTopRankers() bool {
	return len(rr) > 1 && rr[0].rank == rr[1].rank
}

func (rr rankedNames) topRankers() rankedNames {
	if len(rr) == 0 {
		return nil
	}

	toprank := rr[0].rank
	ss := make(rankedNames, 0, 4)
	for _, r := range rr {
		if r.rank == toprank {
			ss = append(ss, r)
		}
	}

	return ss
}

func (rr rankedNames) allNames() string {
	amb := make([][]string, 0, len(rr))
	for _, r := range rr {
		amb = append(amb, r.name)
	}
	return fmt.Sprintf("%+v", amb)
}
