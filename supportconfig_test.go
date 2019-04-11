package supportconfig_test

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhdn/go-supportconfig"
	. "gopkg.in/check.v1"
)

type clientSuite struct {
}

var _ = Suite(&clientSuite{})

func TestNew(t *testing.T) {
}

// Hook up check.v1 into the "go test" runner
func Test(t *testing.T) { TestingT(t) }

func (cs *clientSuite) SetUpTest(c *C) {
	//
}

// UglyExtraNewlines tags places that might need fixing when a proper parser
// is in place
const UglyExtraNewlines = "\n\n"

var sampleCreateParser = `
=============================================================================
                     Support Utilities - Supportconfig
                          Script Version: 3.0-102
                          Script Date: 2017 10 09

 Detailed system information and logs are collected and organized in a
 [...]
=============================================================================

#==[ Command ]======================================#
# /bin/date
Sun Apr  7 20:23:42 CEST 2019
`

func (cs *clientSuite) TestCreateParser(c *C) {
	p := supportconfig.NewParser()
	c.Assert(p, NotNil)
}

func (cs *clientSuite) TestParseEmpty(c *C) {
	p := supportconfig.NewParser()
	err := p.Parse(strings.NewReader(``))
	c.Assert(err, IsNil)
}

type NopWriteCloser struct {
	bytes.Buffer
}

func (NopWriteCloser) Close() error {
	return nil
}

func (cs *clientSuite) TestParseSimpleEvent(c *C) {
	var sectionName, sectionAfter string
	collector := &NopWriteCloser{}
	gotSection := false
	p := supportconfig.NewParser()
	p.HandleSection("Command", func(name, after string) (io.WriteCloser, error) {
		gotSection = true
		sectionName = name
		sectionAfter = after
		return collector, nil
	})
	err := p.Parse(strings.NewReader(sampleCreateParser))
	c.Assert(err, IsNil)
	c.Assert(gotSection, Equals, true)
	c.Assert(sectionName, Equals, "Command")
	c.Assert(sectionAfter, Equals, "# /bin/date")
	c.Assert(collector.String(), Equals, "Sun Apr  7 20:23:42 CEST 2019\n")
}

var sampleMultipleGroups = `
=============================================================================
                     Support Utilities - Supportconfig
                          Script Version: 3.0-102
                          Script Date: 2017 10 09

 Detailed system information and logs are collected and organized in a
 [...]
=============================================================================

#==[ Command ]======================================#
# /bin/date
Sun Apr  7 20:23:42 CEST 2019

#==[ Command ]======================================#
# /bin/uname -a
Linux node 4.4.121-92.85-default #1 SMP Tue Jun 19 07:41:16 UTC 2018 (1fb8a51) x86_64 x86_64 x86_64 GNU/Linux

#==[ Configuration File ]===========================#
# /etc/SuSE-release
SUSE Linux Enterprise Server 12 (x86_64)
VERSION = 12
PATCHLEVEL = 2
# This file is deprecated and will be removed in a future service pack or release.
# Please check /etc/os-release for details about this release.


#==[ System ]=======================================#
# Virtualization
Hardware:      See hardware.txt
Hypervisor:    Xen (/proc/xen)
Identity:      Virtual Machine - DomU (No /proc/xen/xsd_port)
Type:          Fully Virtualized (no xen kernel)
`

const etcRelease = `SUSE Linux Enterprise Server 12 (x86_64)
VERSION = 12
PATCHLEVEL = 2
# This file is deprecated and will be removed in a future service pack or release.
# Please check /etc/os-release for details about this release.
`

const osRelease = `NAME="SLES"
VERSION="12-SP2"
VERSION_ID="12.2"
PRETTY_NAME="SUSE Linux Enterprise Server 12 SP2"
ID="sles"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:suse:sles:12:sp2"
`

func (cs *clientSuite) TestParseMultipleGroups(c *C) {
	var sectionName, sectionAfter string
	collector := &NopWriteCloser{}
	gotSection := false
	p := supportconfig.NewParser()
	p.HandleSection("Configuration File", func(name, after string) (io.WriteCloser, error) {
		gotSection = true
		sectionName = name
		sectionAfter = after
		return collector, nil
	})
	err := p.Parse(strings.NewReader(sampleMultipleGroups))
	c.Assert(err, IsNil)
	c.Assert(gotSection, Equals, true)
	c.Assert(sectionName, Equals, "Configuration File")
	c.Assert(sectionAfter, Equals, "# /etc/SuSE-release")
	c.Assert(collector.String(), Equals, etcRelease+UglyExtraNewlines)
}

var sampleMultipleFiles = `
=============================================================================
                     Support Utilities - Supportconfig
                          Script Version: 3.0-102
                          Script Date: 2017 10 09

 Detailed system information and logs are collected and organized in a
 [...]
=============================================================================

#==[ Command ]======================================#
# /bin/date
Sun Apr  7 20:23:42 CEST 2019

#==[ Command ]======================================#
# /bin/uname -a
Linux node 4.4.121-92.85-default #1 SMP Tue Jun 19 07:41:16 UTC 2018 (1fb8a51) x86_64 x86_64 x86_64 GNU/Linux

#==[ Configuration File ]===========================#
# /etc/SuSE-release
SUSE Linux Enterprise Server 12 (x86_64)
VERSION = 12
PATCHLEVEL = 2
# This file is deprecated and will be removed in a future service pack or release.
# Please check /etc/os-release for details about this release.


#==[ Configuration File ]===========================#
# /etc/os-release
NAME="SLES"
VERSION="12-SP2"
VERSION_ID="12.2"
PRETTY_NAME="SUSE Linux Enterprise Server 12 SP2"
ID="sles"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:suse:sles:12:sp2"


#==[ System ]=======================================#
# Virtualization
Hardware:      See hardware.txt
Hypervisor:    Xen (/proc/xen)
Identity:      Virtual Machine - DomU (No /proc/xen/xsd_port)
Type:          Fully Virtualized (no xen kernel)
`

func (cs *clientSuite) TestSplitterOneFile(c *C) {
	base := c.MkDir()
	gotPath := make([]string, 0)
	handler := func(path string) (string, error) {
		gotPath = append(gotPath, path)
		return path, nil
	}
	config := supportconfig.Config{Base: base, FilenameHandler: handler}
	splitter := &supportconfig.Splitter{Config: config}

	err := splitter.Split(strings.NewReader(sampleMultipleGroups))
	c.Assert(err, IsNil)

	path := "/etc/SuSE-release"
	c.Assert(len(gotPath), Equals, 1)
	c.Assert(gotPath[0], Equals, path)
	b, err := ioutil.ReadFile(filepath.Join(base, path))
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, etcRelease+UglyExtraNewlines)
}

const logEntry = `2011-01-07T11:11:01.111111+02:00 nodename03007.example.net invld>Apr  7 06:50:01 nodename03007 libvirtd[4975]: internal error: missing storage backend for network files using rbd protocol
`

const logEntryWithNote = `
#==[ Log File ]===============================#
# /var/log/nodes/logname.log - Last 10000 Lines
` + logEntry + UglyExtraNewlines

func (cs *clientSuite) TestSplitterWithNote(c *C) {
	base := c.MkDir()
	gotPath := make([]string, 0)
	handler := func(path string) (string, error) {
		gotPath = append(gotPath, path)
		return path, nil
	}
	config := supportconfig.Config{Base: base, FilenameHandler: handler}
	splitter := &supportconfig.Splitter{Config: config}

	err := splitter.Split(strings.NewReader(logEntryWithNote))
	c.Assert(err, IsNil)

	path := "/var/log/nodes/logname.log"
	c.Assert(len(gotPath), Equals, 1)
	c.Assert(gotPath[0], Equals, path)
	b, err := ioutil.ReadFile(filepath.Join(base, path))
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, logEntry+UglyExtraNewlines)
}

const maliciousLogEntry = `
#==[ Log File ]===============================#
# /var/../../../../../.vimrc - Last 10000 Lines
` + logEntry + UglyExtraNewlines

func (cs *clientSuite) TestSplitterWithMaliciousPath(c *C) {
	base := c.MkDir()
	gotPath := make([]string, 0)
	handler := func(path string) (string, error) {
		gotPath = append(gotPath, path)
		return path, nil
	}
	config := supportconfig.Config{Base: base, FilenameHandler: handler}
	splitter := &supportconfig.Splitter{Config: config}

	err := splitter.Split(strings.NewReader(maliciousLogEntry))
	c.Assert(err, IsNil)

	path := "/.vimrc"
	c.Assert(len(gotPath), Equals, 1)
	c.Assert(gotPath[0], Equals, path)
	b, err := ioutil.ReadFile(filepath.Join(base, path))
	c.Assert(err, IsNil)
	c.Assert(string(b), Equals, logEntry+UglyExtraNewlines)
}
