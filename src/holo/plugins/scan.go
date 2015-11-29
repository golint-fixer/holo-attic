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

package plugins

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"../../shared"
	"../common"
	"../files"
)

//Scan discovers entities available for the given entity. Errors are reported
//immediately and will result in nil being returned. "No entities found" will
//be reported as a non-nil empty slice.
//there are no entities.
func (p *Plugin) Scan() common.Entities {
	//plugins with the "built-in" flag do their processing in other scan functions
	switch p.ID() {
	case "files":
		return files.ScanRepo()
	default: //follows below
	}

	//invoke scan operation
	stdout, hadError := p.runScanOperation()
	if hadError {
		return nil
	}

	//parse scan output
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	lineRx := regexp.MustCompile(`^\s*([^:]+): (.+)\s*$`)
	actionRx := regexp.MustCompile(`^([^()]+) \((.+)\)$`)
	report := shared.Report{Action: "scan with plugin", Target: p.ID()}
	hadError = false
	var currentEntity *Entity
	var result common.Entities
	for idx, line := range lines {
		//skip empty lines
		if line == "" {
			continue
		}

		//keep format strings from getting too long
		errorIntro := fmt.Sprintf("error in scan report, line %d", idx+1)

		//general line format is "key: value"
		match := lineRx.FindStringSubmatch(line)
		if match == nil {
			report.AddError("%s: parse error (line was \"%s\")", errorIntro, line)
			hadError = true
			continue
		}
		key, value := match[1], match[2]

		switch {
		case key == "ENTITY":
			//starting new entity
			if currentEntity != nil {
				result = append(result, currentEntity)
			}
			currentEntity = &Entity{plugin: p, id: value, actionVerb: "Working on"}
		case currentEntity == nil:
			//if not, we need to be inside an entity
			//(i.e. line with idx = 0 must start an entity)
			report.AddError("%s: expected entity ID, found attribute \"%s\"", errorIntro, line)
			hadError = true
		case key == "ACTION":
			//parse action verb/reason
			match = actionRx.FindStringSubmatch(value)
			if match == nil {
				currentEntity.actionVerb = value
				currentEntity.actionReason = ""
			} else {
				currentEntity.actionVerb = match[1]
				currentEntity.actionReason = match[2]
			}
		default:
			//store unrecognized keys as info lines
			currentEntity.infoLines = append(currentEntity.infoLines,
				InfoLine{key, value},
			)
		}
	}

	//store last entity
	if currentEntity != nil {
		result = append(result, currentEntity)
	}

	//report errors
	if hadError {
		report.Print()
		return nil
	}

	//on success, ensure non-nil return value
	if result == nil {
		result = common.Entities{}
	}
	return result
}

func (p *Plugin) runScanOperation() (stdout string, hadError bool) {
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err := p.Command([]string{"scan"}, &stdoutBuffer, &stderrBuffer, nil).Run()

	//report any errors or error output
	if err != nil || stderrBuffer.Len() > 0 {
		report := shared.Report{Action: "scan with plugin", Target: p.ID()}
		if err != nil {
			report.AddError(err.Error())
		}
		report.Print()
		fmt.Fprintf(os.Stderr, "\n%s\n\n", strings.TrimSpace(string(stderrBuffer.Bytes())))
	}

	return string(stdoutBuffer.Bytes()), err != nil
}
