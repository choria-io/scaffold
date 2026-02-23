// Copyright (c) 2023-2024, R.I. Pienaar and the Choria Project contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package scaffold renders directory trees from Go or Jet templates.
//
// Templates can be supplied as an on-disk source directory or as an in-memory
// map of filenames to content. The rendered output is written atomically to a
// target directory; only files whose content has changed are copied, making
// repeated renders safe for use with existing targets when MergeTargetDirectory
// is enabled.
//
// Built-in template functions include the Sprig library, a write() function
// that creates additional files from within a template, and a render() function
// that evaluates a partial template from the _partials subdirectory. Custom
// delimiters, post-processing commands, and caller-supplied template functions
// are all supported.
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
	"github.com/Masterminds/sprig/v3"
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

// ManagedFile represents a file and the action that would be taken on it during rendering
type ManagedFile struct {
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

	if len(cfg.Source) > 0 && cfg.SourceDirectory != "" {
		return fmt.Errorf("source and source_directory are mutually exclusive")
	}

	if cfg.SourceDirectory != "" {
		_, err := os.Stat(cfg.SourceDirectory)
		if err != nil {
			return fmt.Errorf("cannot read source directory: %w", err)
		}
	}

	if (cfg.CustomLeftDelimiter == "") != (cfg.CustomRightDelimiter == "") {
		return fmt.Errorf("both left_delimiter and right_delimiter must be set")
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
		s.log.Debugf("Rendered %s", f)
	}

	return nil
}

func (s *Scaffold) renderAndPostFile(out string, t string, data any) error {
	err := s.renderFile(out, t, data)
	switch {
	case errors.Is(err, errSkippedEmpty):
		if s.log != nil {
			s.log.Debugf("Skipping empty file %v", out)
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
		s.log.Debugf("Rendered %s", out)
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
		err := args.ParseInto(&out, &content)
		if err != nil {
			args.Panicf("write: %v", err)
		}

		err = s.saveAndPostFile(filepath.Join(s.cfg.TargetDirectory, out), content)
		if err != nil {
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

	var vm jet.VarMap
	_, ok := data.(jet.VarMap)
	if ok {
		vm = data.(jet.VarMap)
	}

	buf := bytes.NewBuffer([]byte{})
	err = t.Execute(buf, vm, data)
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
				s.log.Debugf("Post processing using: %s %s", cmd, strings.Join(args, " "))
			}

			out, err := exec.Command(cmd, args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to post process %s\nerror: %w\noutput: %q", f, err, out)
			}
		}
	}

	return nil
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

func atomicCopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".scaffold-tmp-")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// clean up temp file on any failure path
		os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmp, srcFile); err != nil {
		tmp.Close()
		return err
	}
	err = tmp.Close()
	if err != nil {
		return err
	}

	err = os.Chmod(tmpName, srcInfo.Mode().Perm())
	if err != nil {
		return err
	}

	return os.Rename(tmpName, dst)
}

func copyTreeToTarget(tmpDir, realTarget string, log Logger) ([]ManagedFile, error) {
	var result []ManagedFile

	err := filepath.WalkDir(tmpDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}

		dst := filepath.Join(realTarget, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0755)
		}

		if d.Type().IsRegular() {
			relSlash := filepath.ToSlash(rel)

			if _, statErr := os.Stat(dst); statErr == nil {
				srcHash, err := sha256File(path)
				if err != nil {
					return err
				}
				dstHash, err := sha256File(dst)
				if err != nil {
					return err
				}
				if srcHash == dstHash {
					if log != nil {
						log.Debugf("Skipping unchanged file %s", rel)
					}
					result = append(result, ManagedFile{Path: relSlash, Action: FileActionEqual})
					return nil
				}

				err = atomicCopyFile(path, dst)
				if err != nil {
					return err
				}
				result = append(result, ManagedFile{Path: relSlash, Action: FileActionUpdate})
				return nil
			}

			err = atomicCopyFile(path, dst)
			if err != nil {
				return err
			}
			result = append(result, ManagedFile{Path: relSlash, Action: FileActionAdd})
		}

		return nil
	})

	return result, err
}

// RenderNoop performs a full render into a temporary directory and compares the
// result against the real target directory. It returns a list of files with
// their planned action (add, update, equal, remove) without modifying the real
// target.
func (s *Scaffold) RenderNoop(data any) ([]ManagedFile, error) {
	realTarget := s.cfg.TargetDirectory

	tmpBase, err := os.MkdirTemp("", "scaffold-noop-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpBase)

	tmpTarget := filepath.Join(tmpBase, "target")

	err = s.renderToDir(tmpTarget, data)
	if err != nil {
		return nil, err
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
	var result []ManagedFile
	for rel, tmpPath := range rendered {
		realPath := filepath.Join(realTarget, filepath.FromSlash(rel))
		_, statErr := os.Stat(realPath)
		if os.IsNotExist(statErr) {
			result = append(result, ManagedFile{Path: rel, Action: FileActionAdd})
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
				result = append(result, ManagedFile{Path: rel, Action: FileActionEqual})
			} else {
				result = append(result, ManagedFile{Path: rel, Action: FileActionUpdate})
			}
		}
	}

	// Walk real target to find files not in rendered output
	if _, err := os.Stat(realTarget); err == nil {
		err = filepath.WalkDir(realTarget, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(realTarget, path)
			if err != nil {
				return err
			}
			relSlash := filepath.ToSlash(rel)
			if _, ok := rendered[relSlash]; !ok {
				result = append(result, ManagedFile{Path: relSlash, Action: FileActionRemove})
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

// renderToDir renders all templates into the specified directory, running
// post-processing on the rendered files. It temporarily sets TargetDirectory
// to dir so that saveFile containment checks and the write() template func
// operate against dir.
func (s *Scaffold) renderToDir(dir string, data any) error {
	origTarget := s.cfg.TargetDirectory
	s.cfg.TargetDirectory = dir
	defer func() { s.cfg.TargetDirectory = origTarget }()

	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	s.workingSource = s.cfg.SourceDirectory

	if s.workingSource == "" {
		s.workingSource, err = s.createTempDirForSource()
		if err != nil {
			return err
		}
		defer func() {
			os.RemoveAll(s.workingSource)
			s.workingSource = ""
		}()
	}

	return filepath.WalkDir(s.workingSource, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == s.workingSource {
			return nil
		}

		if d.Name() == "_partials" {
			return filepath.SkipDir
		}

		out := filepath.Join(dir, strings.TrimPrefix(path, s.workingSource))
		switch {
		case d.IsDir():
			return os.MkdirAll(out, 0755)

		case d.Type().IsRegular():
			return s.renderAndPostFile(out, path, data)

		default:
			return fmt.Errorf("invalid file in source: %v", d.Name())
		}
	})
}

// Render creates the target directory and places all files into it after
// template processing and post-processing. Files are rendered into a temporary
// directory first, then atomically copied to the real target. The returned
// slice describes every managed file and the action taken (add, update, equal).
func (s *Scaffold) Render(data any) ([]ManagedFile, error) {
	tmpDir, err := os.MkdirTemp("", "scaffold-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	tmpTarget := filepath.Join(tmpDir, "target")

	err = s.renderToDir(tmpTarget, data)
	if err != nil {
		return nil, err
	}

	err = os.MkdirAll(s.cfg.TargetDirectory, 0755)
	if err != nil {
		return nil, err
	}

	result, err := copyTreeToTarget(tmpTarget, s.cfg.TargetDirectory, s.log)
	if err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})

	return result, nil
}
