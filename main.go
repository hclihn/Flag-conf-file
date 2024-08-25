package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const ProcessKey = "!!Process!!"

func SliceIterator[T any](slice []T, reverse bool) func() (T, bool) {
	index := 0
	if reverse {
		index = len(slice) - 1
	}
	return func() (item T, ok bool) {
		if reverse {
			if index >= 0 {
				item = slice[index]
				index--
				ok = true
			}
		} else if index < len(slice) {
			item = slice[index]
			index++
			ok = true
		}
		return
	}
}

type CfgStack []map[string]string

func NewCfgStack(fs *flag.FlagSet) (*CfgStack, error) {
	if fs == nil {
		return nil, fmt.Errorf("fs is nil")
	} else if !fs.Parsed() {
		return nil, fmt.Errorf("fs is not parsed")
	}
	m := make(map[string]string)
	fs.Visit(func(f *flag.Flag) {
		m[f.Name] = f.Value.String()
	})
	return &CfgStack{m}, nil
}

func (s *CfgStack) Push(m map[string]string) {
	if s == nil || *s == nil {
		panic("*CfgStack is a nil pointer or pointing to a nil map")
	} else if m == nil {
		return
	}
	*s = append(*s, m)
}

func (s *CfgStack) Unroll(fs *flag.FlagSet, lateSet bool) {
	if s == nil || *s == nil {
		panic("*CfgStack is a nil pointer or pointing to a nil map")
	}
	var res map[string]string
	var rest CfgStack
	l := len(*s)
	if lateSet {
		res, rest = (*s)[l-1], (*s)[:l-1]
	} else {
		res, rest = (*s)[0], (*s)[1:]
	}
	iterator := SliceIterator(rest, lateSet)
	for item, ok := iterator(); ok; item, ok = iterator() {
		for k, v := range item {
			if _, ok := res[k]; !ok {
				res[k] = v
			}
		}
	}	
	delete(res, ProcessKey)
	for k, v := range res {
		fmt.Printf("--> Unroll: Set(%s, %s)\n", k, v)
		if err := fs.Set(k, v); err != nil {
			fmt.Printf("Failed to set flag %q to value %q: %v\n", k, v, err)
		}
	}
}

func (s *CfgStack) ProcessCfgFile(path, cfgName string, fs *flag.FlagSet) error {
	fmt.Printf("Processing config %q...\n", path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Error reading config file %s: %w", path, err)
	}
	cmds := strings.Split(strings.TrimSpace(strings.ReplaceAll(
			string(data), "\n", " ")), " ")
	fs.Parse(cmds)
	m := make(map[string]string)
	m[ProcessKey] = path
	var cfgFile string
	fs.Visit(func(f *flag.Flag) {
		fmt.Printf("-> Flag=%q, Value.String=%q, Default=%q\n", f.Name, f.Value, f.DefValue)
		for _, cmd := range cmds {
			if strings.HasPrefix(cmd, "-") && strings.HasSuffix(cmd, f.Name) {
				fmt.Printf("-> m[%s] = %q\n", f.Name, f.Value)
				m[f.Name] = f.Value.String()
				if f.Name == cfgName {
					cfgFile = f.Value.String()
				}
				break
			}
		}
	})
	s.Push(m)
	if cfgFile != "" {
		for _, mm := range *s {
			if mm[ProcessKey] == cfgFile {
				return fmt.Errorf("circular reference detected: %s", cfgFile)
			}
		}
		if err := s.ProcessCfgFile(cfgFile, cfgName, fs); err != nil {
			return fmt.Errorf("failed to process config file %s: %w", cfgFile, err)
		}
	}
	return nil
}

func main() {
	var name, cfgFile string
	var age int
	var male bool
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	fs.StringVar(&cfgFile, "c", "", "Read flags from `FILE`")
	fs.StringVar(&name, "n", "", "Specify your `NAME`")
	fs.IntVar(&age, "a", 0, "Specify your `AGE`")
	fs.BoolVar(&male, "m", false, "Are you a male?")
	fs.Usage()
	fs.Parse([]string{"-n", "fake", "-c", "test.cfg"})
	fmt.Printf("After Parse: name=%q, cfgFile=%q, age=%d, male=%v\n", name, cfgFile, age, male)
	
	if cfgFile != "" {
		stack, err := NewCfgStack(fs) 
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			os.Exit(1)
		}
		if err := stack.ProcessCfgFile(cfgFile, "c", fs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Printf("Before Unroll: name=%q, cfgFile=%q, age=%d, male=%v\n", name, cfgFile, age, male)
		stack.Unroll(fs, false)
		fmt.Printf("After Unroll EarlySet: name=%q, cfgFile=%q, age=%d, male=%v\n", name, cfgFile, age, male)
		stack.Unroll(fs, true)
		fmt.Printf("After Unroll LateSet: name=%q, cfgFile=%q, age=%d, male=%v\n", name, cfgFile, age, male)
		stack.Unroll(fs, false)
		fmt.Printf("After Unroll EarlySet#2: name=%q, cfgFile=%q, age=%d, male=%v\n", name, cfgFile, age, male)
	}
}
