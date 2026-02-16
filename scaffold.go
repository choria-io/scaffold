// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

package scaffold

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/CloudyKit/jet/v6"
	"github.com/choria-io/scaffold/internal/sprig"
	"github.com/kballard/go-shellquote"
)

// Config configures a scaffolding operation
type Config struct {
	// TargetDirectory is where to place the resulting rendered files, must not exist
	TargetDirectory string `yaml:"target"`
	// SourceDirectory reads templates from a directory, mutually exclusive with Source
	SourceDirectory string `yaml:"source_directory"`
	// MergeTargetDirectory writes into existing target directories
	MergeTargetDirectory bool `yaml:"merge_target_directory"`
	// Source reads templates from in-process memory
	Source map[string]any `yaml:"source"`
	// Post configures post-processing of files using filepath globs
	Post []map[string]string `yaml:"post"`
	// SkipEmpty skips files that are 0 bytes after rendering
	SkipEmpty bool `yaml:"skip_empty"`
	// Sets a custom template delimiter, useful for generating templates from templates
	CustomLeftDelimiter string `yaml:"left_delimiter"`
	// Sets a custom template delimiter, useful for generating templates from templates
	CustomRightDelimiter string `yaml:"right_delimiter"`
}

type Logger interface {
	Debugf(format string, v ...any)
	Infof(format string, v ...any)
}

var errSkippedEmpty = errors.New("skipped rendering")

type engineType int

const (
	engineGoTemplate engineType = iota
	engineJet
)

// FileAction represents the type of change a file would undergo during rendering
type FileAction string

const (
	FileActionAdd    FileAction = "add"
	FileActionUpdate FileAction = "update"
	FileActionEqual  FileAction = "equal"
	FileActionRemove FileAction = "remove"
)

// PlannedFile represents a file and the action that would be taken on it during rendering
type PlannedFile struct {
	Path   string
	Action FileAction
}

type Scaffold struct {
	cfg           *Config
	engine        engineType
	funcs         template.FuncMap
	jetFuncs      map[string]jet.Func
	log           Logger
	workingSource string
	currentDir    string
	changedFiles  []string
}

// New creates a new scaffold instance
func New(cfg Config, funcs template.FuncMap) (*Scaffold, error) {
	err := validateConfig(&cfg)
	if err != nil {
		return nil, err
	}

	return &Scaffold{cfg: &cfg, funcs: funcs}, nil
}

// NewJet creates a new scaffold instance using the Jet template engine
func NewJet(cfg Config, funcs map[string]jet.Func) (*Scaffold, error) {
	err := validateConfig(&cfg)
	if err != nil {
		return nil, err
	}

	return &Scaffold{cfg: &cfg, engine: engineJet, jetFuncs: funcs}, nil
}

func validateConfig(cfg *Config) error {
	if cfg.TargetDirectory == "" {
		return fmt.Errorf("target is required")
	}

	var err error
	cfg.TargetDirectory, err = filepath.Abs(cfg.TargetDirectory)
	if err != nil {
		return fmt.Errorf("invalid target %s: %v", cfg.TargetDirectory, err)
	}

	if len(cfg.Source) == 0 && cfg.SourceDirectory == "" {
		return fmt.Errorf("no sources provided")
	}

	if cfg.SourceDirectory != "" {
		_, err := os.Stat(cfg.SourceDirectory)
		if err != nil {
			return fmt.Errorf("cannot read source directory: %w", err)
		}
	}

	if !cfg.MergeTargetDirectory {
		_, err := os.Stat(cfg.TargetDirectory)
		if err == nil {
			return fmt.Errorf("target directory exists")
		}
	}

	return nil
}

// RenderString renders a string using the same functions and behavior as the scaffold, including custom delimiters
func (s *Scaffold) RenderString(str string, data any) (string, error) {
	res, err := s.renderTemplateBytes("string", []byte(str), data)
	if err != nil {
		return "", err
	}

	return string(res), nil
}

// Logger configures a logger to use, no logging is done without this
func (s *Scaffold) Logger(log Logger) {
	s.log = log
}

func (s *Scaffold) dumpSourceDir(source map[string]any, target string) error {
	for k, v := range source {
		if strings.Contains(k, "..") {
			return fmt.Errorf("invalid file name %v", k)
		}
		if strings.ContainsAny(k, `/\`) {
			return fmt.Errorf("invalid file name %v", k)
		}

		out := filepath.Join(target, k)

		switch e := v.(type) {
		case string: // a file
			err := os.WriteFile(out, []byte(e), 0400)
			if err != nil {
				return err
			}

		case map[string]any: // a directory
			err := os.Mkdir(out, 0700)
			if err != nil {
				return err
			}

			err = s.dumpSourceDir(e, out)
			if err != nil {
				return err
			}

		default: // a mistake
			return fmt.Errorf("invalid source entry %s: %v", k, v)
		}
	}

	return nil
}

func (s *Scaffold) createTempDirForSource() (string, error) {
	td, err := os.MkdirTemp("", "")
	if err != nil {
		return "", err
	}

	err = s.dumpSourceDir(s.cfg.Source, td)
	if err != nil {
		os.RemoveAll(td)
		return "", err
	}

	return td, nil
}

func (s *Scaffold) saveAndPostFile(f string, data string) error {
	err := s.saveFile(f, data)
	if err != nil {
		return err
	}

	err = s.postFile(f)
	if err != nil {
		return err
	}

	if s.log != nil {
		s.log.Infof("Rendered %s", f)
	}

	return nil
}

func (s *Scaffold) renderAndPostFile(out string, t string, data any) error {
	err := s.renderFile(out, t, data)
	switch {
	case errors.Is(err, errSkippedEmpty):
		if s.log != nil {
			s.log.Infof("Skipping empty file %v", out)
		}

		return nil
	case err != nil:
		return err
	}

	err = s.postFile(out)
	if err != nil {
		return err
	}

	if s.log != nil {
		s.log.Infof("Rendered %s", out)
	}

	return nil
}

func (s *Scaffold) templateFuncs() template.FuncMap {
	funcs := sprig.FuncMap()
	for k, v := range s.funcs {
		funcs[k] = v
	}

	funcs["write"] = func(out string, content string) (string, error) {
		err := s.saveAndPostFile(filepath.Join(s.cfg.TargetDirectory, out), content)
		return "", err
	}

	funcs["render"] = func(templ string, data any) (string, error) {
		path, err := s.validateSourcePath(templ)
		if err != nil {
			return "", err
		}
		res, err := s.renderTemplateFile(path, data)
		return string(res), err
	}

	return funcs
}

func (s *Scaffold) jetTemplateFuncs() map[string]jet.Func {
	funcs := make(map[string]jet.Func)
	for k, v := range s.jetFuncs {
		funcs[k] = v
	}

	funcs["write"] = func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("write", 2, 2)

		var out, content string
		if err := args.ParseInto(&out, &content); err != nil {
			args.Panicf("write: %v", err)
		}

		if err := s.saveAndPostFile(filepath.Join(s.cfg.TargetDirectory, out), content); err != nil {
			args.Panicf("write: %v", err)
		}

		return reflect.ValueOf("")
	}

	funcs["render"] = func(args jet.Arguments) reflect.Value {
		args.RequireNumOfArguments("render", 2, 2)

		templ := args.Get(0).String()
		data := args.Get(1).Interface()

		path, err := s.validateSourcePath(templ)
		if err != nil {
			args.Panicf("render: %v", err)
		}

		res, err := s.renderTemplateFile(path, data)
		if err != nil {
			args.Panicf("render: %v", err)
		}

		return reflect.ValueOf(string(res))
	}

	return funcs
}

func (s *Scaffold) renderTemplateFile(tmpl string, data any) ([]byte, error) {
	td, err := os.ReadFile(tmpl)
	if err != nil {
		return nil, err
	}

	return s.renderTemplateBytes(filepath.Base(tmpl), td, data)
}

func (s *Scaffold) renderTemplateBytes(name string, tmpl []byte, data any) ([]byte, error) {
	switch s.engine {
	case engineJet:
		return s.renderTemplateBytesJet(name, tmpl, data)
	default:
		return s.renderTemplateBytesGoTempl(name, tmpl, data)
	}
}

func (s *Scaffold) renderTemplateBytesGoTempl(name string, tmpl []byte, data any) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	templ := template.New(name)
	funcs := s.templateFuncs()
	if funcs != nil {
		templ.Funcs(funcs)
	}

	if s.cfg.CustomLeftDelimiter != "" && s.cfg.CustomRightDelimiter != "" {
		templ.Delims(s.cfg.CustomLeftDelimiter, s.cfg.CustomRightDelimiter)
	}

	templ, err := templ.Parse(string(tmpl))
	if err != nil {
		return nil, fmt.Errorf("parsing template %v failed: %w", name, err)
	}

	err = templ.Execute(buf, data)
	if err != nil {
		return nil, err
	}

	if s.cfg.SkipEmpty && len(bytes.TrimSpace(buf.Bytes())) == 0 {
		return nil, errSkippedEmpty
	}

	return buf.Bytes(), nil
}

func (s *Scaffold) renderTemplateBytesJet(name string, tmpl []byte, data any) ([]byte, error) {
	loader := jet.NewInMemLoader()
	loader.Set(name, string(tmpl))

	opts := []jet.Option{jet.WithSafeWriter(nil)}
	if s.cfg.CustomLeftDelimiter != "" && s.cfg.CustomRightDelimiter != "" {
		opts = append(opts, jet.WithDelims(s.cfg.CustomLeftDelimiter, s.cfg.CustomRightDelimiter))
	}

	set := jet.NewSet(loader, opts...)

	for k, fn := range s.jetTemplateFuncs() {
		set.AddGlobalFunc(k, fn)
	}

	t, err := set.GetTemplate(name)
	if err != nil {
		return nil, fmt.Errorf("parsing template %v failed: %w", name, err)
	}

	buf := bytes.NewBuffer([]byte{})
	err = t.Execute(buf, nil, data)
	if err != nil {
		return nil, err
	}

	if s.cfg.SkipEmpty && len(bytes.TrimSpace(buf.Bytes())) == 0 {
		return nil, errSkippedEmpty
	}

	return buf.Bytes(), nil
}

func containedInDir(path string, dir string) bool {
	return path == dir || strings.HasPrefix(path, dir+string(filepath.Separator))
}

func (s *Scaffold) validateSourcePath(name string) (string, error) {
	path := filepath.Join(s.workingSource, name)

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid source path %s: %v", name, err)
	}

	absSource, err := filepath.Abs(s.workingSource)
	if err != nil {
		return "", fmt.Errorf("invalid source directory: %v", err)
	}

	if !containedInDir(absPath, absSource) {
		return "", fmt.Errorf("%s is not in source directory %s", name, s.workingSource)
	}

	return path, nil
}

func (s *Scaffold) saveFile(out string, content string) error {
	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}

	if !containedInDir(absOut, s.cfg.TargetDirectory) {
		return fmt.Errorf("%s is not in target directory %s", out, s.cfg.TargetDirectory)
	}

	err = os.WriteFile(out, []byte(content), 0644)
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(s.cfg.TargetDirectory, absOut)
	if err != nil {
		return err
	}
	s.changedFiles = append(s.changedFiles, filepath.ToSlash(rel))

	return nil
}

func (s *Scaffold) renderFile(out string, t string, data any) error {
	res, err := s.renderTemplateFile(t, data)
	if err != nil {
		return err
	}

	return s.saveFile(out, string(res))
}

func (s *Scaffold) postFile(f string) error {
	for _, p := range s.cfg.Post {
		for g, v := range p {
			matched, err := filepath.Match(g, filepath.Base(f))
			if err != nil {
				return err
			}

			if !matched {
				continue
			}

			parts, err := shellquote.Split(v)
			if err != nil {
				return err
			}

			cmd := parts[0]
			var args []string
			hasPlaceholder := false
			for _, p := range parts[1:] {
				if strings.Contains(p, "{}") {
					args = append(args, strings.ReplaceAll(p, "{}", f))
					hasPlaceholder = true
				} else {
					args = append(args, p)
				}
			}

			if !hasPlaceholder {
				args = append(args, f)
			}

			if s.log != nil {
				s.log.Infof("Post processing using: %s %s", cmd, strings.Join(args, " "))
			}

			out, err := exec.Command(cmd, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to post process %s\nerror: %w\noutput: %q", f, err, out)
			}
		}
	}

	return nil
}

// ChangedFiles returns the list of files that were created or modified during
// the most recent Render call. Paths are relative to the target directory and
// always use forward slashes as separators.
func (s *Scaffold) ChangedFiles() []string {
	return s.changedFiles
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// RenderNoop performs a full render into a temporary directory and compares the
// result against the real target directory. It returns a list of files with
// their planned action (add, update, equal, remove) without modifying the real
// target. The caller's ChangedFiles state is preserved.
func (s *Scaffold) RenderNoop(data any) ([]PlannedFile, error) {
	origTarget := s.cfg.TargetDirectory
	origMerge := s.cfg.MergeTargetDirectory
	origChanged := s.changedFiles

	tmpBase, err := os.MkdirTemp("", "scaffold-noop-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpBase)

	tmpTarget := filepath.Join(tmpBase, "target")
	s.cfg.TargetDirectory = tmpTarget
	s.cfg.MergeTargetDirectory = false

	renderErr := s.Render(data)

	s.cfg.TargetDirectory = origTarget
	s.cfg.MergeTargetDirectory = origMerge
	s.changedFiles = origChanged

	if renderErr != nil {
		return nil, renderErr
	}

	// Build set of rendered file paths
	rendered := map[string]string{} // relative slash path -> absolute path in temp
	err = filepath.WalkDir(tmpTarget, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(tmpTarget, path)
		if err != nil {
			return err
		}
		rendered[filepath.ToSlash(rel)] = path
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Compare rendered files against real target
	var result []PlannedFile
	for rel, tmpPath := range rendered {
		realPath := filepath.Join(origTarget, filepath.FromSlash(rel))
		_, statErr := os.Stat(realPath)
		if os.IsNotExist(statErr) {
			result = append(result, PlannedFile{Path: rel, Action: FileActionAdd})
		} else if statErr != nil {
			return nil, statErr
		} else {
			tmpHash, err := sha256File(tmpPath)
			if err != nil {
				return nil, err
			}
			realHash, err := sha256File(realPath)
			if err != nil {
				return nil, err
			}
			if tmpHash == realHash {
				result = append(result, PlannedFile{Path: rel, Action: FileActionEqual})
			} else {
				result = append(result, PlannedFile{Path: rel, Action: FileActionUpdate})
			}
		}
	}

	// Walk real target to find files not in rendered output
	if _, err := os.Stat(origTarget); err == nil {
		err = filepath.WalkDir(origTarget, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(origTarget, path)
			if err != nil {
				return err
			}
			relSlash := filepath.ToSlash(rel)
			if _, ok := rendered[relSlash]; !ok {
				result = append(result, PlannedFile{Path: relSlash, Action: FileActionRemove})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result, nil
}

// Render creates the target directory and place all files into it after template processing and post-processing
func (s *Scaffold) Render(data any) error {
	s.changedFiles = nil

	err := os.MkdirAll(s.cfg.TargetDirectory, 0755)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	err = os.Chdir(s.cfg.TargetDirectory)
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)

	s.workingSource = s.cfg.SourceDirectory

	if s.workingSource == "" {
		// move the memory source to temp dir
		s.workingSource, err = s.createTempDirForSource()
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(s.workingSource)
			s.workingSource = ""
		}()
	}

	s.currentDir = s.cfg.TargetDirectory
	defer func() { s.currentDir = "" }()

	// now render both the same way
	err = filepath.WalkDir(s.workingSource, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == s.workingSource {
			return nil
		}

		if d.Name() == "_partials" {
			return filepath.SkipDir
		}

		out := filepath.Join(s.cfg.TargetDirectory, strings.TrimPrefix(path, s.workingSource))
		switch {
		case d.IsDir():
			err := os.Mkdir(out, 0755)
			if err != nil {
				return err
			}

		case d.Type().IsRegular():
			s.currentDir = filepath.Dir(out)
			err = s.renderAndPostFile(out, path, data)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("invalid file in source: %v", d.Name())
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
