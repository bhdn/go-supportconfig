package supportconfig

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/runc/libcontainer/utils"
)

// HandlerFunc is the func that is used by HandleSection
type HandlerFunc func(section, next string) (io.WriteCloser, error)

// Parser keeps the state of the parsing accross different files
type Parser struct {
	handlers map[string][]HandlerFunc
}

// NewParser initialiazes a new Parser
func NewParser() *Parser {
	parser := &Parser{handlers: make(map[string][]HandlerFunc)}
	return parser
}

// Parse starts reading the source and triggers the events when sections
// are matched.
func (p *Parser) Parse(source io.Reader) error {
	var section, afterSection string
	var collectors []io.WriteCloser

	re := regexp.MustCompile(`#==\[ (.*?) \]=+`)
	scanner := bufio.NewScanner(source)

section:
	for _, collector := range collectors {
		collector.Close()
	}
	collectors = nil
	afterSection = ""
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.HasPrefix(line, []byte("#==[ ")) {
			found := re.FindSubmatchIndex(line)
			if len(found) > 0 {
				begin, end := found[2], found[3]
				section = string(line[begin:end])
				goto section
			}
		} else if section != "" {
			if afterSection == "" {
				afterSection = string(line)
				collectors = make([]io.WriteCloser, 0)
				for _, handler := range p.handlers[section] {
					if collector, err := handler(section, afterSection); err != nil {
						if err == SkipFile {
							continue

						}
						return err
					} else {
						collectors = append(collectors, collector)
					}
				}
			} else {
				for _, collector := range collectors {
					collector.Write(line)
					collector.Write([]byte("\n"))
				}
			}
		}
	}
	if len(collectors) > 0 {
		goto section
	}

	return nil
}

// HandleSection adds a handler to a given slice of handlers for the
// section found
func (p *Parser) HandleSection(section string, handler HandlerFunc) {
	p.handlers[section] = append(p.handlers[section], handler)
}

// PathHandlerFunc says to the splitter what is the filename to be used
// for a given path
type PathHandlerFunc func(path string) (newpath string, err error)

// Config has settings for the file splitter
type Config struct {

	// Base destination directory
	Base string

	// FilenameFunc gets a path as in the source file and should return
	// the destination path (later to be joined with the base
	// directory)
	PathHandler PathHandlerFunc
}

// Splitter has the state of the splitter
type Splitter struct {
	Config Config
}

var InvalidEntry = fmt.Errorf("Invalid entry in the source file")
var SkipFile = fmt.Errorf("This file must be skipped")

const FileNotFound = "File not found"

func afterlineToPath(afterline string) (string, error) {
	if idx := strings.LastIndex(afterline, FileNotFound); idx > -1 {
		return "", SkipFile
	} else if strings.HasSuffix(afterline, " Lines") {
		idx := strings.LastIndex(afterline, " - ")
		if idx > 0 {
			afterline = afterline[:idx]
		}
	}
	return afterline, nil
}

func (s *Splitter) handler(section, afterline string) (io.WriteCloser, error) {
	var err error
	var dest, origDest string

	const prefix = "# "
	if !strings.HasPrefix(afterline, prefix) {
		return nil, InvalidEntry
	}
	origDest, err = afterlineToPath(afterline[len(prefix):])
	if err != nil {
		return nil, SkipFile
	}
	origDest = utils.CleanPath(origDest)
	if origDest == "" {
		return nil, InvalidEntry
	}

	if s.Config.PathHandler != nil {
		dest, err = s.Config.PathHandler(origDest)
		if err != nil {
			return nil, err
		}
	} else {
		dest = origDest
	}

	path := filepath.Join(s.Config.Base, dest)
	base := filepath.Dir(path)

	err = os.MkdirAll(base, os.ModePerm)
	if err != nil {
		return nil, err
	}

	writer := bufio.NewWriter(f)
	nop := &NopWriteCloser{f: f}
	nop.Writer = *writer

	return nop, nil
}

type NopWriteCloser struct {
	bufio.Writer
	f *os.File
}

func (n *NopWriteCloser) Close() error {
	n.Flush()
	return n.f.Close()
}

// Runs the splitter for a reable source
func (s *Splitter) Split(source io.Reader) error {
	p := NewParser()

	for _, name := range []string{"Configuration File", "Log File"} {
		p.HandleSection(name, s.handler)
	}

	return p.Parse(source)
}
