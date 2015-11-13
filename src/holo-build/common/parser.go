/*******************************************************************************
*
* Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
* This file is part of Holo.
*
* Holo is free software: you can redistribute it and/or modify it under the
* terms of the GNU General Public License as published by the Free Software
* Foundation, either version 3 of the License, or (at your option) any later
* version.
*
* Holo is distributed in the hope that it will be useful, but WITHOUT ANY
* WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR
* A PARTICULAR PURPOSE. See the GNU General Public License for more details.
*
* You should have received a copy of the GNU General Public License along with
* Holo. If not, see <http://www.gnu.org/licenses/>.
*
*******************************************************************************/

package common

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"../../internal/toml"
)

//PackageDefinition only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type PackageDefinition struct {
	Package   PackageSection
	File      []FileSection
	Directory []DirectorySection
	Symlink   []SymlinkSection
}

//PackageSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type PackageSection struct {
	Name          string
	Version       string
	Release       uint
	Epoch         uint
	Description   string
	Author        string
	Requires      []string
	Provides      []string
	Conflicts     []string
	Replaces      []string
	SetupScript   string
	CleanupScript string
}

//FileSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type FileSection struct {
	Path        string
	Content     string
	ContentFrom string
	Raw         bool
	Mode        string      //TOML does not support octal number literals, so we have to write: mode = "0666"
	Owner       interface{} //either string (name) or integer (ID)
	Group       interface{} //same
	//NOTE: We could use custom types implementing TextUnmarshaler for Mode,
	//Owner and Group, but then toml.Decode would accept any primitive type.
	//But for Mode, we need the type enforcement to prevent the "mode = 0666"
	//error (which would be 666 in decimal = something else in octal). And for
	//Owner and Group, we need to distinguish IDs from names using the type.
}

//DirectorySection only needs a nice exported name for the TOML parser to
//produce more meaningful error messages on malformed input data.
type DirectorySection struct {
	Path  string
	Mode  string      //see above
	Owner interface{} //see above
	Group interface{} //see above
}

//SymlinkSection only needs a nice exported name for the TOML parser to produce
//more meaningful error messages on malformed input data.
type SymlinkSection struct {
	Path   string
	Target string
}

//versions are dot-separated numbers like (0|[1-9][0-9]*) (this enforces no
//trailing zeros)
var versionRx = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.(?:0|[1-9][0-9]*))*$`)

//the author information should be in the form "Firstname Lastname <email.address@server.tld>"
var authorRx = regexp.MustCompile(`^[^<>]+\s+<[^<>\s]+>$`)

//ParsePackageDefinition parses a package definition from the given input.
//The operation is successful if the returned []error is nil or empty.
func ParsePackageDefinition(input io.Reader) (*Package, []error) {
	//read from input
	blob, err := ioutil.ReadAll(input)
	if err != nil {
		return nil, []error{err}
	}
	var p PackageDefinition
	_, err = toml.Decode(string(blob), &p)
	if err != nil {
		return nil, []error{err}
	}

	//restructure the parsed data into a common.Package struct
	fsEntryCount := len(p.Directory) + len(p.File) + len(p.Symlink)
	pkg := Package{
		Name:          strings.TrimSpace(p.Package.Name),
		Version:       strings.TrimSpace(p.Package.Version),
		Release:       p.Package.Release,
		Epoch:         p.Package.Epoch,
		Description:   strings.TrimSpace(p.Package.Description),
		Author:        strings.TrimSpace(p.Package.Author),
		SetupScript:   strings.TrimSpace(p.Package.SetupScript),
		CleanupScript: strings.TrimSpace(p.Package.CleanupScript),
		FSEntries:     make([]FSEntry, 0, fsEntryCount),
	}

	//default value for Release is 1
	if pkg.Release == 0 {
		pkg.Release = 1
	}

	//do some basic validation on the package name and version since we're
	//going to use these to construct a path
	ec := &ErrorCollector{}
	switch {
	case pkg.Name == "":
		ec.Addf("Missing package name")
	case strings.ContainsAny(pkg.Name, "/\r\n"):
		ec.Addf("Invalid package name \"%s\" (may not contain slashes or newlines)", pkg.Name)
	}
	switch {
	case pkg.Version == "":
		ec.Addf("Missing package version")
	case !versionRx.MatchString(pkg.Version):
		ec.Addf("Invalid package version \"%s\" (must be a chain of numbers like \"1.2.0\" or \"20151104\")", pkg.Version)
	}
	if strings.ContainsAny(pkg.Description, "\r\n") {
		ec.Addf("Invalid package description \"%s\" (may not contain newlines)", pkg.Name)
	}
	//the author field is not required (except for --debian), but if it is
	//given, check the format
	if pkg.Author != "" && !authorRx.MatchString(pkg.Author) {
		ec.Addf("Invalid package author \"%s\" (should look like \"Jane Doe <jane.doe@example.org>\")", pkg.Author)
	}

	//parse relations to other packages
	pkg.Requires = parseRelatedPackages("requires", p.Package.Requires, ec)
	pkg.Provides = parseRelatedPackages("provides", p.Package.Provides, ec)
	pkg.Conflicts = parseRelatedPackages("conflicts", p.Package.Conflicts, ec)
	pkg.Replaces = parseRelatedPackages("replaces", p.Package.Replaces, ec)

	//parse and validate FS entries
	wasPathSeen := make(map[string]bool, fsEntryCount)

	for idx, dirSection := range p.Directory {
		path := dirSection.Path
		validatePath(path, &wasPathSeen, ec, "directory", idx)

		entryDesc := fmt.Sprintf("directory \"%s\"", path)
		pkg.FSEntries = append(pkg.FSEntries, FSEntry{
			Type:  FSEntryTypeDirectory,
			Path:  path,
			Mode:  parseFileMode(dirSection.Mode, 0755, ec, entryDesc),
			Owner: parseUserOrGroupRef(dirSection.Owner, ec, entryDesc),
			Group: parseUserOrGroupRef(dirSection.Group, ec, entryDesc),
		})
	}

	for idx, fileSection := range p.File {
		path := fileSection.Path
		validatePath(path, &wasPathSeen, ec, "file", idx)

		entryDesc := fmt.Sprintf("file \"%s\"", path)
		pkg.FSEntries = append(pkg.FSEntries, FSEntry{
			Type:    FSEntryTypeRegular,
			Path:    path,
			Content: parseFileContent(fileSection.Content, fileSection.ContentFrom, fileSection.Raw, ec, entryDesc),
			Mode:    parseFileMode(fileSection.Mode, 0644, ec, entryDesc),
			Owner:   parseUserOrGroupRef(fileSection.Owner, ec, entryDesc),
			Group:   parseUserOrGroupRef(fileSection.Group, ec, entryDesc),
		})
	}

	for idx, symlinkSection := range p.Symlink {
		path := symlinkSection.Path
		validatePath(path, &wasPathSeen, ec, "symlink", idx)

		if symlinkSection.Target == "" {
			ec.Addf("symlink \"%s\" is invalid: missing target", path)
		}

		pkg.FSEntries = append(pkg.FSEntries, FSEntry{
			Type:    FSEntryTypeSymlink,
			Path:    path,
			Content: symlinkSection.Target,
		})
	}

	return &pkg, ec.Errors
}

//relatedPackageRx and providesPackageRx are nearly identical, except that for a "provides" relation, only the operator "=" is acceptable
var relatedPackageRx = regexp.MustCompile(`^([^\s<=>]+)\s*(?:(<=?|>=?|=)\s*([^\s<=>]+))?$`)
var providesPackageRx = regexp.MustCompile(`^([^\s<=>]+)\s*(?:(=)\s*([^\s<=>]+))?$`)

func parseRelatedPackages(relType string, specs []string, ec *ErrorCollector) []PackageRelation {
	rels := make([]PackageRelation, 0, len(specs))
	idxByName := make(map[string]int, len(specs))

	for _, spec := range specs {
		//which format to use?
		rx := relatedPackageRx
		if relType == "provides" {
			rx = providesPackageRx
		}

		//check format of spec
		match := rx.FindStringSubmatch(spec)
		if match == nil {
			ec.Addf("Invalid package reference in %s: \"%s\"", relType, spec)
			continue
		}

		//do we have a relation to this package already?
		name := match[1]
		idx, exists := idxByName[name]
		if !exists {
			//no, add a new one and remember it for later additional constraints
			idx = len(rels)
			idxByName[name] = idx
			rels = append(rels, PackageRelation{RelatedPackage: name})
		}

		//add version constraint if one was specified
		if match[2] != "" {
			constraint := VersionConstraint{Relation: match[2], Version: match[3]}
			rels[idx].Constraints = append(rels[idx].Constraints, constraint)
		}
	}

	return rels
}

//path is the path to be validated.
//wasPathSeen tracks usage of paths to detect duplicate entries.
//ec collects errors.
//entryType and entryIdx are
func validatePath(path string, wasPathSeen *map[string]bool, ec *ErrorCollector, entryType string, entryIdx int) bool {
	if path == "" {
		ec.Addf("%s %d is invalid: missing \"path\" attribute", entryType, entryIdx)
		return false
	}
	if !strings.HasPrefix(path, "/") {
		ec.Addf("%s \"%s\" is invalid: must be an absolute path", entryType, path)
		return false
	}
	if strings.HasSuffix(path, "/") {
		ec.Addf("%s \"%s\" is invalid: trailing slash(es)", entryType, path)
		return false
	}
	if (*wasPathSeen)[path] {
		ec.Addf("multiple entries for path \"%s\"", path)
		return false
	}
	(*wasPathSeen)[path] = true
	return true
}

func parseFileMode(modeStr string, defaultMode os.FileMode, ec *ErrorCollector, entryDesc string) os.FileMode {
	//default value
	if modeStr == "" {
		return defaultMode
	}

	//parse modeStr as uint in base 8 to uint32 (== os.FileMode)
	value, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		ec.Addf("%s is invalid: cannot parse mode \"%s\" (%s)", entryDesc, modeStr, err.Error())
	}
	return os.FileMode(value)
}

//this regexp copied from useradd(8) manpage
var userOrGroupRx = regexp.MustCompile(`^[a-z_][a-z0-9_-]*\$?$`)

func parseUserOrGroupRef(value interface{}, ec *ErrorCollector, entryDesc string) *IntOrString {
	//default value
	if value == nil {
		return nil
	}

	switch val := value.(type) {
	case int64:
		if val < 0 {
			ec.Addf("%s is invalid: user or group ID \"%d\" may not be negative", entryDesc, val)
		}
		if val >= 1<<32 {
			ec.Addf("%s is invalid: user or group ID \"%d\" does not fit in uint32", entryDesc, val)
		}
		return &IntOrString{Int: uint32(val)}
	case string:
		if !userOrGroupRx.MatchString(val) {
			ec.Addf("%s is invalid: \"%s\" is not an acceptable user or group name", entryDesc, val)
		}
		return &IntOrString{Str: val}
	default:
		ec.Addf("%s is invalid: \"owner\"/\"group\" attributes must be strings or integers, found type %T", entryDesc, value)
		return nil
	}
}

func parseFileContent(content string, contentFrom string, dontPruneIndent bool, ec *ErrorCollector, entryDesc string) string {
	//option 1: content given verbatim in "content" field
	if content != "" {
		if contentFrom != "" {
			ec.Addf("%s is invalid: cannot use both `content` and `contentFrom`", entryDesc)
		}
		if dontPruneIndent {
			return content
		}
		return string(pruneIndentation([]byte(content)))
	}

	//option 2: content referenced in "contentFrom" field
	if contentFrom == "" {
		ec.Addf("%s is invalid: missing content", entryDesc)
		return ""
	}
	bytes, err := ioutil.ReadFile(contentFrom)
	ec.Add(err)
	return string(bytes)
}

func pruneIndentation(text []byte) []byte {
	//split into lines for analysis
	lines := bytes.Split(text, []byte{'\n'})

	//use the indentation of the first non-empty line as a starting point for the longest common prefix
	var prefix []byte
	for _, line := range lines {
		if len(line) != 0 {
			lineWithoutIndentation := bytes.TrimLeft(line, "\t ")
			prefix = line[:len(line)-len(lineWithoutIndentation)]
			break
		}
	}

	//find the longest common prefix (from the starting point, remove trailing
	//characters until it *is* the longest common prefix)
	for len(prefix) > 0 {
		found := true
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, prefix) {
				//not the longest common prefix yet -> chop off one byte and retry
				prefix = prefix[:len(prefix)-1]
				found = false
				break
			}
		}
		if found {
			break
		}
	}

	//remove the longest common prefix from all non-empty lines
	if len(prefix) == 0 {
		return text //fast exit
	}
	for idx, line := range lines {
		if len(line) > 0 {
			lines[idx] = line[len(prefix):]
		}
	}
	return bytes.Join(lines, []byte{'\n'})
}
