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

package entities

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"../common"
)

//Group represents a UNIX group (as registered in /etc/group). It implements
//the Entity interface and is handled accordingly.
type Group struct {
	name            string   //the group name (the first field in /etc/group)
	gid             int      //the GID (the third field in /etc/group), or 0 if no specific GID is enforced
	system          bool     //whether the group is a system group (this influences the GID selection if gid = 0)
	definitionFiles []string //paths to the files defining this entity

	broken bool //whether the entity definition is invalid (default: false)
}

//isValid is used inside the scanning algorithm to filter entities with
//broken definitions, which shall be skipped during `holo apply`.
func (g *Group) isValid() bool { return !g.broken }

//setInvalid is used inside the scnaning algorithm to mark entities with
//broken definitions, which shall be skipped during `holo apply`.
func (g *Group) setInvalid() { g.broken = true }

//EntityID implements the Entity interface for Group.
func (g Group) EntityID() string { return "group:" + g.name }

//Report implements the Entity interface for Group.
func (g Group) Report() *common.Report {
	r := common.Report{Target: g.EntityID()}
	for _, defFile := range g.definitionFiles {
		r.AddLine("found in", defFile)
	}
	if attributes := g.attributes(); attributes != "" {
		r.AddLine("with", attributes)
	}
	return &r
}

func (g Group) attributes() string {
	attrs := []string{}
	if g.system {
		attrs = append(attrs, "type: system")
	}
	if g.gid > 0 {
		attrs = append(attrs, fmt.Sprintf("GID: %d", g.gid))
	}
	return strings.Join(attrs, ", ")
}

//Apply performs the complete application algorithm for the given Entity.
//If the group does not exist yet, it is created. If it does exist, but some
//attributes do not match, it will be updated, but only if withForce is given.
func (g Group) Apply(withForce bool) {
	r := g.Report()
	r.Action = "Working on"
	entityHasChanged := g.doApply(r, withForce)
	if entityHasChanged {
		r.Print()
	} else {
		r.PrintUnlessEmpty()
	}
}

type groupDiff struct {
	field    string
	actual   string
	expected string
}

func (g Group) doApply(report *common.Report, withForce bool) (entityHasChanged bool) {
	//check if we have that group already
	groupExists, actualGid, err := g.checkExists()
	if err != nil {
		report.AddError("Cannot read group database: %s", err.Error())
		return false
	}

	//check if the actual properties diverge from our definition
	if groupExists {
		differences := []groupDiff{}
		if g.gid > 0 && g.gid != actualGid {
			differences = append(differences, groupDiff{"GID", strconv.Itoa(actualGid), strconv.Itoa(g.gid)})
		}

		if len(differences) != 0 {
			if withForce {
				for _, diff := range differences {
					report.AddLine("fix", fmt.Sprintf("%s (was: %s)", diff.field, diff.actual))
				}
				err := g.callGroupmod(report)
				if err != nil {
					report.AddError(err.Error())
					return false
				}
				return true
			}
			for _, diff := range differences {
				report.AddError("Group has %s: %s, expected %s (use --force to overwrite)", diff.field, diff.actual, diff.expected)
			}
		}
		return false
	}

	//create the group if it does not exist
	err = g.callGroupadd(report)
	if err != nil {
		report.AddError(err.Error())
		return false
	}
	return true
}

func (g Group) checkExists() (exists bool, gid int, e error) {
	groupFile := filepath.Join(common.TargetDirectory(), "etc/group")

	//fetch entry from /etc/group
	fields, err := common.Getent(groupFile, func(fields []string) bool { return fields[0] == g.name })
	if err != nil {
		return false, 0, err
	}
	//is there such a group?
	if fields == nil {
		return false, 0, nil
	}
	//is the group entry intact?
	if len(fields) < 4 {
		return true, 0, errors.New("invalid entry in /etc/group (not enough fields)")
	}

	//read fields in entry
	actualGid, err := strconv.Atoi(fields[2])
	return true, actualGid, err
}

func (g Group) callGroupadd(report *common.Report) error {
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
	_, err := common.ExecProgramOrMock(report, []byte{}, "groupadd", args...)
	return err
}

func (g Group) callGroupmod(report *common.Report) error {
	//assemble arguments for groupmod call
	args := []string{}
	if g.gid > 0 {
		args = append(args, "--gid", strconv.Itoa(g.gid))
	}
	args = append(args, g.name)

	//call groupmod
	_, err := common.ExecProgramOrMock(report, []byte{}, "groupmod", args...)
	return err
}