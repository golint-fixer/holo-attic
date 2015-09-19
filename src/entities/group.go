/*******************************************************************************
*
*   Copyright 2015 Stefan Majewsky <majewsky@gmx.net>
*
*   This program is free software; you can redistribute it and/or modify it
*   under the terms of the GNU General Public License as published by the Free
*   Software Foundation; either version 2 of the License, or (at your option)
*   any later version.
*
*   This program is distributed in the hope that it will be useful, but WITHOUT
*   ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
*   FITNESS FOR A PARTICULAR PURPOSE.  See the GNU General Public License for
*   more details.
*
*   You should have received a copy of the GNU General Public License along
*   with this program; if not, write to the Free Software Foundation, Inc.,
*   51 Franklin Street, Fifth Floor, Boston, MA 02110-1301, USA.
*
********************************************************************************/

package entities

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"../common"
)

//Group represents a UNIX group (as registered in /etc/group). It implements
//the Entity interface and is handled accordingly.
type Group struct {
	name           string
	gid            int
	system         bool
	definitionFile string
}

//Name returns the group name (the first field in /etc/group).
func (g Group) Name() string { return g.name }

//NumericID returns the GID (the third field in /etc/group).
func (g Group) NumericID() int { return g.gid }

//System returns true if the group shall be created as a system group.
func (g Group) System() bool { return g.system }

//EntityID implements the Entity interface for Group.
func (g Group) EntityID() string { return "group:" + g.name }

//DefinitionFile implements the Entity interface for Group.
func (g Group) DefinitionFile() string { return g.definitionFile }

//Attributes implements the Entity interface for Group.
func (g Group) Attributes() string {
	attrs := []string{}
	if g.system {
		attrs = append(attrs, "type: system")
	}
	if g.gid > 0 {
		attrs = append(attrs, fmt.Sprintf("gid: %d", g.gid))
	}
	return strings.Join(attrs, ", ")
}

//Apply performs the complete application algorithm for the givne Entity.
//If the group does not exist yet, it is created. If it does exist, but some
//attributes do not match, it will be updated, but only if withForce is given.
func (g Group) Apply(withForce bool) {
	common.PrintInfo("Working on \x1b[1m%s\x1b[0m", g.EntityID())

	//check if we have that group already
	groupExists, actualGid, err := g.checkExists()
	if err != nil {
		common.PrintError("Error encountered while reading /etc/group: %s", err.Error())
		return
	}

	//check if the actual properties diverge from our definition
	if groupExists {
		errors := []string{}
		if g.gid > 0 && g.gid != actualGid {
			errors = append(errors, fmt.Sprintf("GID: %d, expected %d", actualGid, g.gid))
		}

		if len(errors) != 0 {
			if withForce {
				common.PrintInfo("       fix %s", strings.Join(errors, ", "))
				g.callGroupmod()
			} else {
				common.PrintWarning("       has %s (use --force to overwrite)", strings.Join(errors, ", "))
			}
		}
	} else {
		//create the group if it does not exist
		description := g.Attributes()
		if description != "" {
			description = "with " + description
		}
		common.PrintInfo("    create group %s", description)
		g.callGroupadd()
	}
}

func (g Group) checkExists() (exists bool, gid int, err error) {
	//read /etc/group
	contents, err := ioutil.ReadFile(filepath.Join(common.TargetDirectory(), "etc/group"))
	if err != nil {
		return false, 0, err
	}

	//find the line that defines this group
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	for _, line := range lines {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if fields[0] == g.name {
			//group found - check GID field
			gid, err := strconv.Atoi(fields[2])
			return true, gid, err
		}
	}

	//there is no group with that name
	return false, 0, nil
}

func (g Group) callGroupadd() {
	//assemble arguments for groupadd call
	args := []string{}
	if g.system {
		args = append(args, "--system")
	}
	if g.gid > 0 {
		args = append(args, "--gid", strconv.Itoa(g.gid))
	}
	args = append(args, g.name)

	//call groupadd
	_, err := common.ExecProgramOrMock([]byte{}, "groupadd", args...)
	if err != nil {
		common.PrintError(err.Error())
	}
}

func (g Group) callGroupmod() {
	//assemble arguments for groupmod call
	args := []string{}
	if g.gid > 0 {
		args = append(args, "--gid", strconv.Itoa(g.gid))
	}
	args = append(args, g.name)

	//call groupmod
	_, err := common.ExecProgramOrMock([]byte{}, "groupmod", args...)
	if err != nil {
		common.PrintError(err.Error())
	}
}